// Package surefire reads Maven Surefire and Failsafe XML.
// It counts testcase elements. It does not trust suite totals.
// SUREFIRE-1627 can count each rerun as another test in those totals.
package surefire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Outcome values match the test_run check constraint.
const (
	Passed  = "PASSED"
	Failed  = "FAILED"
	Error   = "ERROR"
	Skipped = "SKIPPED"
	Flaky   = "FLAKY"
)

// Case is one test method. Module is filled by the caller from the file path.
// The parser does not know which Maven module wrote the file.
type Case struct {
	ClassName     string
	MethodName    string
	Outcome       string
	DurationMs    int64
	RerunFailures int32
	FailureType   string
	FailureHash   string
}

// frameworkPrefixes are not the subject's own code. The first frame outside
// this list is the one hashed, so the same assertion hashes across builds.
var frameworkPrefixes = []string{
	"java.",
	"javax.",
	"jdk.",
	"sun.",
	"com.sun.",
	"org.junit.",
	"junit.",
	"org.hamcrest.",
	"org.opentest4j.",
	"org.testng.",
	"org.apache.maven.",
	"org.gradle.",
}

type testCaseXML struct {
	Name         string      `xml:"name,attr"`
	ClassName    string      `xml:"classname,attr"`
	Time         string      `xml:"time,attr"`
	Failure      *detailXML  `xml:"failure"`
	Error        *detailXML  `xml:"error"`
	Skipped      *skippedXML `xml:"skipped"`
	FlakyFailure []detailXML `xml:"flakyFailure"`
	FlakyError   []detailXML `xml:"flakyError"`
	RerunFailure []detailXML `xml:"rerunFailure"`
	RerunError   []detailXML `xml:"rerunError"`
}

type skippedXML struct {
	Message string `xml:"message,attr"`
}

type detailXML struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

// Parse reads one Surefire or Failsafe file. An empty suite is not an error.
// A testcase with no class or method name is an error: the schema would reject
// it, and a silent skip would hide a bad report.
func Parse(r io.Reader) ([]Case, error) {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return nil, fmt.Errorf("unsupported xml charset %q", charset)
	}
	var out []Case
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("xml: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "testcase" {
			continue
		}
		var raw testCaseXML
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return nil, fmt.Errorf("testcase: %w", err)
		}
		one, err := raw.caseResult()
		if err != nil {
			return nil, err
		}
		out = append(out, one)
	}
}

func (raw testCaseXML) caseResult() (Case, error) {
	className := strings.TrimSpace(raw.ClassName)
	methodName := strings.TrimSpace(raw.Name)
	if className == "" || methodName == "" {
		return Case{}, fmt.Errorf("testcase missing classname or name")
	}
	duration, err := durationMs(raw.Time)
	if err != nil {
		return Case{}, fmt.Errorf("%s#%s: %w", className, methodName, err)
	}
	got := Case{
		ClassName:  className,
		MethodName: methodName,
		DurationMs: duration,
		Outcome:    Passed,
	}
	switch {
	case raw.Error != nil:
		got.Outcome = Error
		got.RerunFailures = int32(len(raw.RerunFailure) + len(raw.RerunError))
		applyDetail(&got, raw.Error)
	case raw.Failure != nil:
		got.Outcome = Failed
		got.RerunFailures = int32(len(raw.RerunFailure) + len(raw.RerunError))
		applyDetail(&got, raw.Failure)
	case len(raw.FlakyFailure)+len(raw.FlakyError) > 0:
		got.Outcome = Flaky
		got.RerunFailures = int32(len(raw.FlakyFailure) + len(raw.FlakyError))
		detail := firstDetail(raw.FlakyError, raw.FlakyFailure)
		applyDetail(&got, detail)
	case raw.Skipped != nil:
		got.Outcome = Skipped
	}
	return got, nil
}

func firstDetail(primary, secondary []detailXML) *detailXML {
	if len(primary) > 0 {
		return &primary[0]
	}
	if len(secondary) > 0 {
		return &secondary[0]
	}
	return nil
}

func applyDetail(got *Case, detail *detailXML) {
	if detail == nil {
		return
	}
	failureType := strings.TrimSpace(detail.Type)
	if failureType == "" {
		failureType = firstToken(detail.Body)
	}
	got.FailureType = failureType
	frame := firstApplicationFrame(detail.Body)
	material := failureType
	if frame != "" {
		material += "\n" + frame
	} else if msg := strings.TrimSpace(detail.Message); msg != "" {
		material += "\n" + msg
	}
	if material == "" {
		return
	}
	sum := sha256.Sum256([]byte(material))
	got.FailureHash = hex.EncodeToString(sum[:])
}

func durationMs(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("time %q: %w", raw, err)
	}
	if seconds < 0 {
		return 0, fmt.Errorf("time %q is negative", raw)
	}
	return int64(math.Round(seconds * 1000)), nil
}

func firstToken(body string) string {
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return ""
	}
	token := fields[0]
	if i := strings.Index(token, ":"); i > 0 {
		return token[:i]
	}
	return token
}

func firstApplicationFrame(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "at ") {
			continue
		}
		if isFramework(classOf(line)) {
			continue
		}
		return line
	}
	return ""
}

func classOf(frame string) string {
	frame = strings.TrimSpace(strings.TrimPrefix(frame, "at "))
	if i := strings.Index(frame, "("); i >= 0 {
		frame = frame[:i]
	}
	if i := strings.LastIndex(frame, "."); i >= 0 {
		frame = frame[:i]
	}
	if i := strings.Index(frame, "/"); i >= 0 {
		frame = frame[i+1:]
	}
	return frame
}

func isFramework(className string) bool {
	for _, prefix := range frameworkPrefixes {
		if strings.HasPrefix(className, prefix) {
			return true
		}
	}
	return false
}

// ModuleFromFilename returns the Maven module in a path such as
// "core/target/surefire-reports/TEST-Foo.xml". A report at the workspace root
// has no module. The module is stored and is not part of the test identity.
func ModuleFromFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	const marker = "/target/"
	i := strings.Index(name, marker)
	if i <= 0 {
		return ""
	}
	mod := name[:i]
	if slash := strings.LastIndex(mod, "/"); slash >= 0 {
		mod = mod[slash+1:]
	}
	if mod == "." || strings.Contains(mod, "..") || strings.TrimSpace(mod) == "" {
		return ""
	}
	return mod
}

// StageFromFilename reports QUARANTINE when the CLI preserved the quarantine
// report directory. Every other report is the blocking stage.
func StageFromFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.Contains(name, "/quarantine-reports/") {
		return "QUARANTINE"
	}
	return "BLOCKING"
}

// ParseBytes is Parse for a fixture or an uploaded part.
func ParseBytes(b []byte) ([]Case, error) {
	return Parse(bytes.NewReader(b))
}
