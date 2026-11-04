package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"h-ui/util"
)

type blockingLogWriter struct {
	started chan struct{}
	release chan struct{}
}

func (w *blockingLogWriter) Write(p []byte) (int, error) {
	close(w.started)
	<-w.release
	return len(p), nil
}

func testLogSpec(t *testing.T) logSpec {
	t.Helper()
	return logSpec{Type: LogTypeAccess, Path: filepath.Join(t.TempDir(), "hy2_access.log")}
}

func writeTestLog(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0640); err != nil {
		t.Fatal(err)
	}
}

func TestReadLogStreamIncrementalWithoutDuplicates(t *testing.T) {
	spec := testLogSpec(t)
	writeTestLog(t, spec.Path, "one\ntwo\n")
	first, err := readLogStream(spec, "", 200)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first.Lines, ",") != "one,two" {
		t.Fatalf("initial lines = %v", first.Lines)
	}
	file, err := os.OpenFile(spec.Path, os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("partial")
	_ = file.Close()
	second, err := readLogStream(spec, first.Cursor, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Lines) != 0 {
		t.Fatalf("partial line was returned: %v", second.Lines)
	}
	file, _ = os.OpenFile(spec.Path, os.O_APPEND|os.O_WRONLY, 0640)
	_, _ = file.WriteString("-done\nthree\n")
	_ = file.Close()
	third, err := readLogStream(spec, second.Cursor, 200)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(third.Lines, ",") != "partial-done,three" {
		t.Fatalf("incremental lines = %v", third.Lines)
	}
}

func TestReadLogStreamReturnsLatest200(t *testing.T) {
	spec := testLogSpec(t)
	var content strings.Builder
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&content, "line-%03d\n", i)
	}
	writeTestLog(t, spec.Path, content.String())
	result, err := readLogStream(spec, "", 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != 200 || result.Lines[0] != "line-050" || result.Lines[199] != "line-249" {
		t.Fatalf("unexpected tail: len=%d first=%q last=%q", len(result.Lines), result.Lines[0], result.Lines[len(result.Lines)-1])
	}
}

func TestReadLogStreamResetsAfterRotationOrTruncate(t *testing.T) {
	spec := testLogSpec(t)
	writeTestLog(t, spec.Path, "old-first\nold-second\n")
	first, err := readLogStream(spec, "", 200)
	if err != nil {
		t.Fatal(err)
	}
	writeTestLog(t, spec.Path, "new-first\nnew-second\nnew-third\n")
	result, err := readLogStream(spec, first.Cursor, 200)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reset || strings.Join(result.Lines, ",") != "new-first,new-second,new-third" {
		t.Fatalf("rotation reset = %v lines = %v", result.Reset, result.Lines)
	}
}

func TestLogUsageIncludesOnlyActiveAndRotatedFiles(t *testing.T) {
	spec := testLogSpec(t)
	writeTestLog(t, spec.Path, strings.Repeat("a", 10))
	writeTestLog(t, filepath.Join(filepath.Dir(spec.Path), "hy2_access-2026-10-31T12-00-00.000.log"), strings.Repeat("b", 20))
	writeTestLog(t, filepath.Join(filepath.Dir(spec.Path), "hy2_access-2026-10-31T12-00-01.000.log.gz"), strings.Repeat("c", 30))
	writeTestLog(t, filepath.Join(filepath.Dir(spec.Path), "hy2_access.log.backup"), strings.Repeat("x", 100))
	usage, err := measureLogUsage(spec)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Bytes != 60 || usage.Files != 3 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestLogUsageReportsUnreadableDirectoryAsError(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	writeTestLog(t, parent, "x")
	usage, err := measureLogUsage(logSpec{Type: LogTypeAccess, Path: filepath.Join(parent, "hy2_access.log")})
	if err == nil {
		t.Fatal("unreadable log directory was reported as zero usage")
	}
	if usage.Bytes != 0 || usage.Exists {
		t.Fatalf("unexpected usage on error: %+v", usage)
	}
}

func TestClearLogTruncatesActiveRemovesHistoryAndKeepsWriterUsable(t *testing.T) {
	spec := testLogSpec(t)
	writeTestLog(t, spec.Path, "before\n")
	rotated := filepath.Join(filepath.Dir(spec.Path), "hy2_access-2026-10-31T12-00-00.000.log.gz")
	writeTestLog(t, rotated, "history")
	unrelated := filepath.Join(filepath.Dir(spec.Path), "database.db")
	writeTestLog(t, unrelated, "keep")
	writer, err := os.OpenFile(spec.Path, os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if err = clearLog(spec); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(spec.Path); info.Size() != 0 {
		t.Fatalf("active log size = %d", info.Size())
	}
	if _, err = os.Stat(rotated); !os.IsNotExist(err) {
		t.Fatalf("rotated log still exists: %v", err)
	}
	if _, err = os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated file was removed: %v", err)
	}
	if _, err = writer.WriteString("after\n"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(spec.Path)
	if string(data) != "after\n" {
		t.Fatalf("active writer content = %q", data)
	}
}

func TestClearLogWaitsForConcurrentWriter(t *testing.T) {
	spec := testLogSpec(t)
	writeTestLog(t, spec.Path, "before\n")
	blocking := &blockingLogWriter{started: make(chan struct{}), release: make(chan struct{})}
	writer := util.NewSynchronizedWriter(spec.Path, blocking)
	writeDone := make(chan struct{})
	go func() {
		_, _ = writer.Write([]byte("during\n"))
		close(writeDone)
	}()
	<-blocking.started
	clearDone := make(chan error, 1)
	go func() { clearDone <- clearLog(spec) }()
	select {
	case err := <-clearDone:
		t.Fatalf("clear completed while writer held the file lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(blocking.release)
	<-writeDone
	if err := <-clearDone; err != nil {
		t.Fatal(err)
	}
}

func TestReadLogStreamRejectsInvalidCursor(t *testing.T) {
	spec := testLogSpec(t)
	writeTestLog(t, spec.Path, "line\n")
	if _, err := readLogStream(spec, "not-a-cursor", 200); err == nil {
		t.Fatal("invalid cursor was accepted")
	}
}
