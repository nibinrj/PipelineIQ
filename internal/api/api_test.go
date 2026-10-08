package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nibinrj/PipelineIQ/internal/ingest"
)

func TestCreateAndAuth(t *testing.T) {
	const key = "local-key"
	fake := &fakeIngest{}
	mux := http.NewServeMux()
	Register(mux, nil, key, fake, nil)

	meta := `{"repository":"nibinrj/pipelineiq-lab","job_name":"pipelineiq-lab/main","build_number":7,"branch":"main","commit_sha":"abc","result":"SUCCESS"}`
	body, contentType := multipartBody(t, meta, "core/target/surefire-reports/TEST-Core.xml", passedXML)

	tests := []struct {
		name       string
		auth       string
		wantStatus int
		wantApply  bool
	}{
		{name: "success", auth: "Bearer " + key, wantStatus: http.StatusOK, wantApply: true},
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "wrong", auth: "Bearer other", wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake.applied = nil
			req := httptest.NewRequest(http.MethodPost, "/api/v1/builds", bytes.NewReader(body))
			req.Header.Set("Content-Type", contentType)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantApply {
				if fake.applied == nil || fake.applied.JobName != "pipelineiq-lab/main" {
					t.Fatalf("apply = %+v", fake.applied)
				}
				if len(fake.applied.Tests) != 1 || fake.applied.Tests[0].Module != "core" {
					t.Fatalf("tests = %+v", fake.applied.Tests)
				}
				if strings.Contains(rec.Body.String(), "postgres") {
					t.Fatalf("response leaked a driver detail: %s", rec.Body.String())
				}
			} else if fake.applied != nil {
				t.Fatal("unauthorized request called Apply")
			}
		})
	}
}

func TestGetNotFound(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, nil, "k", &fakeIngest{getErr: ingest.ErrNotFound}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/builds/4", nil)
	req.Header.Set("Authorization", "Bearer k")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
}

func multipartBody(t *testing.T, meta, filename, xml string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("metadata", meta); err != nil {
		t.Fatal(err)
	}
	part, err := w.CreateFormFile("report", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(xml)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

const passedXML = `<?xml version="1.0"?><testsuite tests="1"><testcase name="adds" classname="io.pipelineiq.lab.core.CoreTest" time="0.001"/></testsuite>`

type fakeIngest struct {
	applied *ingest.Report
	getErr  error
}

func (f *fakeIngest) Apply(_ context.Context, report ingest.Report) (ingest.Build, error) {
	f.applied = &report
	return ingest.Build{ID: 1, Repository: report.Repository, JobName: report.JobName, BuildNumber: report.BuildNumber, Result: report.Result, Stages: []ingest.StageView{}, Tests: []ingest.TestView{}}, nil
}

func (f *fakeIngest) Get(context.Context, int64) (ingest.Build, error) {
	if f.getErr != nil {
		return ingest.Build{}, f.getErr
	}
	return ingest.Build{ID: 4, Stages: []ingest.StageView{}, Tests: []ingest.TestView{}}, nil
}

func TestResponseIsJSON(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, nil, "k", &fakeIngest{}, nil)
	body, contentType := multipartBody(t, `{"repository":"r","job_name":"j","build_number":1,"branch":"main","commit_sha":"abc","result":"SUCCESS"}`, "TEST.xml", passedXML)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/builds", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer k")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var view ingest.Build
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.ID != 1 {
		t.Fatalf("id = %d", view.ID)
	}
}
