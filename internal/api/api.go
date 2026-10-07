// Package api is the ingest HTTP surface. Health checks stay in httpserver.
// This package is the only one that knows the multipart field names.
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nibinrj/PipelineIQ/internal/ingest"
	"github.com/nibinrj/PipelineIQ/internal/surefire"
)

// MaxBody is the largest upload accepted. The service container is 64 MB.
const MaxBody = 8 << 20

// Ingester is what the handlers need. Tests pass a fake. The real type is
// *ingest.Service, which opens Postgres.
type Ingester interface {
	Apply(ctx context.Context, report ingest.Report) (ingest.Build, error)
	Get(ctx context.Context, id int64) (ingest.Build, error)
}

// Register mounts the ingest routes on the process mux.
func Register(mux *http.ServeMux, log *slog.Logger, key string, svc Ingester) {
	if log == nil {
		log = slog.Default()
	}
	h := &handler{log: log, key: key, svc: svc}
	mux.Handle("POST /api/v1/builds", h.auth(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/v1/builds/{id}", h.auth(http.HandlerFunc(h.get)))
}

type handler struct {
	log *slog.Logger
	key string
	svc Ingester
}

func (h *handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r.Header.Get("Authorization"), h.key) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authorized(header, key string) bool {
	const prefix = "Bearer "
	if len(header) < len(prefix) || subtle.ConstantTimeCompare([]byte(header[:len(prefix)]), []byte(prefix)) != 1 {
		return false
	}
	got := header[len(prefix):]
	if len(got) != len(key) || key == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(key)) == 1
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBody)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected multipart form"})
		return
	}
	report, err := readReport(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	view, err := h.svc.Apply(r.Context(), report)
	if err != nil {
		h.log.Error("ingest failed", "err", err, "job", report.JobName, "build", report.BuildNumber)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "build id must be a positive integer"})
		return
	}
	view, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if ingest.IsNotFound(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		h.log.Error("get build failed", "err", err, "id", id)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, view)
}

type metadata struct {
	Repository  string     `json:"repository"`
	JobName     string     `json:"job_name"`
	BuildNumber int32      `json:"build_number"`
	Branch      string     `json:"branch"`
	PRNumber    *int32     `json:"pr_number"`
	CommitSHA   string     `json:"commit_sha"`
	Result      string     `json:"result"`
	AgentName   string     `json:"agent_name"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	DurationMs  *int64     `json:"duration_ms"`
}

type stageFile struct {
	Name       string     `json:"name"`
	StartedAt  *time.Time `json:"started_at"`
	DurationMs int64      `json:"duration_ms"`
	Result     string     `json:"result"`
}

func readReport(r *http.Request) (ingest.Report, error) {
	raw := r.FormValue("metadata")
	if raw == "" {
		return ingest.Report{}, fmt.Errorf("metadata is required")
	}
	var meta metadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return ingest.Report{}, fmt.Errorf("metadata: %w", err)
	}
	report := ingest.Report{
		Repository:  strings.TrimSpace(meta.Repository),
		JobName:     strings.TrimSpace(meta.JobName),
		BuildNumber: meta.BuildNumber,
		Branch:      strings.TrimSpace(meta.Branch),
		PRNumber:    meta.PRNumber,
		CommitSHA:   strings.TrimSpace(meta.CommitSHA),
		Result:      strings.TrimSpace(meta.Result),
		AgentName:   strings.TrimSpace(meta.AgentName),
		StartedAt:   meta.StartedAt,
		FinishedAt:  meta.FinishedAt,
		DurationMs:  meta.DurationMs,
	}
	if stages := r.FormValue("stages"); stages != "" {
		var parsed []stageFile
		if err := json.Unmarshal([]byte(stages), &parsed); err != nil {
			return ingest.Report{}, fmt.Errorf("stages: %w", err)
		}
		for _, stage := range parsed {
			report.Stages = append(report.Stages, ingest.Stage{
				Name:       strings.TrimSpace(stage.Name),
				StartedAt:  stage.StartedAt,
				DurationMs: stage.DurationMs,
				Result:     strings.TrimSpace(stage.Result),
			})
		}
	}
	if r.MultipartForm != nil {
		if files := r.MultipartForm.File["log_tail"]; len(files) > 0 {
			text, err := readPart(files[0])
			if err != nil {
				return ingest.Report{}, fmt.Errorf("log_tail: %w", err)
			}
			report.LogTail = text
		}
		for _, header := range r.MultipartForm.File["report"] {
			name := uploadedName(header)
			body, err := readPart(header)
			if err != nil {
				return ingest.Report{}, fmt.Errorf("report %s: %w", name, err)
			}
			cases, err := surefire.ParseBytes([]byte(body))
			if err != nil {
				return ingest.Report{}, fmt.Errorf("report %s: %w", name, err)
			}
			// ParseMultipartForm keeps only the base name. The module is the
			// directory before /target/, so the original relative path is required.
			module := surefire.ModuleFromFilename(name)
			for _, one := range cases {
				report.Tests = append(report.Tests, ingest.Test{Module: module, Case: one})
			}
		}
	}
	return report, nil
}

func uploadedName(header *multipart.FileHeader) string {
	if header == nil {
		return ""
	}
	_, params, err := mime.ParseMediaType(header.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		return params["filename"]
	}
	return header.Filename
}

func readPart(header *multipart.FileHeader) (string, error) {
	f, err := header.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, MaxBody))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
