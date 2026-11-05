package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixedAccessLogger(t *testing.T) (*accessLogger, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", "hy2_access.log")
	l := newAccessLogger(path)
	l.now = func() time.Time {
		return time.Date(2026, 10, 31, 12, 5, 31, 870371000, time.UTC)
	}
	t.Cleanup(func() { _ = l.close() })
	return l, path
}

func readAccessLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAccessLoggerTCPRequestUsesRealEventFields(t *testing.T) {
	l, path := fixedAccessLogger(t)
	l.consume(`{"level":"debug","msg":"TCP request","addr":"1.83.222.29:26220","id":"alice","reqAddr":"4.213.25.240:443"}`)

	got := readAccessLog(t, path)
	want := "2026/10/31 12:05:31.870371 from 1.83.222.29:26220 requested tcp:4.213.25.240:443 [HY2_IN -> DIRECT] user: alice\n"
	if got != want {
		t.Fatalf("access log = %q, want %q", got, want)
	}
}

func TestAccessLoggerParsesHysteriaConsoleEvent(t *testing.T) {
	l, path := fixedAccessLogger(t)
	l.consume("2026-10-31T20:36:02+08:00\tWARN\tTCP error\t{\"addr\":\"1.83.222.29:26220\",\"id\":\"claude\",\"reqAddr\":\"4.213.25.240:443\",\"error\":\"dial tcp4 4.213.25.240:443: i/o timeout\"}")

	got := readAccessLog(t, path)
	if !strings.Contains(got, "error tcp:4.213.25.240:443 [HY2_IN -> DIRECT] user: claude") ||
		!strings.Contains(got, "error: dial tcp4 4.213.25.240:443: i/o timeout") {
		t.Fatalf("unexpected console event access log: %s", got)
	}
}

func TestAccessLoggerParsesHUIWrappedConsoleEvent(t *testing.T) {
	l, path := fixedAccessLogger(t)
	l.consume(`{"level":"error","msg":"2026-10-31T20:36:02+08:00\tDEBUG\tTCP request\t{\"addr\":\"1.83.222.29:26220\",\"id\":\"claude\",\"reqAddr\":\"example.com:443\"}"}`)

	got := readAccessLog(t, path)
	if !strings.Contains(got, "requested tcp:example.com:443 [HY2_IN -> DIRECT] user: claude") {
		t.Fatalf("unexpected wrapped console event access log: %s", got)
	}
}

func TestAccessLoggerInitializeCreatesDirectoryAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "var", "lib", "hui", "hy2_access.log")
	l := newAccessLogger(path)
	if err := l.initialize(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.close() })
	if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
		t.Fatalf("access log directory was not initialized: %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("access log file was not initialized: %v", err)
	}
}

func TestAccessLoggerInitializeReportsFailure(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	l := newAccessLogger(filepath.Join(parent, "hy2_access.log"))
	if err := l.initialize(); err == nil {
		t.Fatal("initialize returned nil for an invalid log directory")
	}
}

func TestAccessLoggerUDPIPv6AndSessionCorrelation(t *testing.T) {
	l, path := fixedAccessLogger(t)
	l.consume(`{"level":"debug","msg":"UDP request","addr":"[2001:db8::1]:26221","id":"bob","sessionID":17,"reqAddr":"[2606:4700:4700::1111]:53"}`)
	l.consume(`{"level":"warn","msg":"UDP error","addr":"[2001:db8::1]:26221","id":"bob","sessionID":17,"error":"network unreachable"}`)

	got := readAccessLog(t, path)
	if !strings.Contains(got, "requested udp:[2606:4700:4700::1111]:53") ||
		!strings.Contains(got, "error udp:[2606:4700:4700::1111]:53") ||
		!strings.Contains(got, "user: bob error: network unreachable") {
		t.Fatalf("unexpected UDP access log:\n%s", got)
	}
}

func TestAccessLoggerIgnoresQUICConnectionLogs(t *testing.T) {
	l, path := fixedAccessLogger(t)
	l.consume(`{"level":"info","msg":"client connected","addr":"1.2.3.4:1234","id":"alice","tx":100}`)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("non-request event created access log: %v", err)
	}
}

func TestAccessLoggerSanitizesFields(t *testing.T) {
	l, path := fixedAccessLogger(t)
	l.consume(`{"msg":"TCP error","addr":"1.2.3.4:5","id":"alice\nadmin","reqAddr":"example.com:443","error":"dial\tfailed"}`)
	got := readAccessLog(t, path)
	if strings.ContainsAny(strings.TrimSuffix(got, "\n"), "\r\n\t") {
		t.Fatalf("access log contains injected control characters: %q", got)
	}
}

func TestAccessLoggerConcurrentWrites(t *testing.T) {
	l, path := fixedAccessLogger(t)
	const count = 100
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l.consume(fmt.Sprintf(`{"msg":"TCP request","addr":"1.2.3.4:%d","id":"user-%d","reqAddr":"example.com:443"}`, 1000+i, i))
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(readAccessLog(t, path)), "\n")
	if len(lines) != count {
		t.Fatalf("line count = %d, want %d", len(lines), count)
	}
	format := regexp.MustCompile(`^2026/10/31 12:05:31\.870371 from .+ requested tcp:example\.com:443 \[HY2_IN -> DIRECT\] user: user-\d+$`)
	for _, line := range lines {
		if !format.MatchString(line) {
			t.Fatalf("malformed concurrent line: %q", line)
		}
	}
}

func TestAccessLoggerAppendsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hy2_access.log")
	for _, user := range []string{"alice", "bob"} {
		l := newAccessLogger(path)
		l.consume(fmt.Sprintf(`{"msg":"TCP request","addr":"1.2.3.4:5","id":%q,"reqAddr":"example.com:443"}`, user))
		if err := l.close(); err != nil {
			t.Fatal(err)
		}
	}
	if lines := strings.Count(readAccessLog(t, path), "\n"); lines != 2 {
		t.Fatalf("line count = %d, want 2", lines)
	}
}

func TestAccessLoggerRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hy2_access.log")
	l := newAccessLogger(path)
	l.writerCfg.maxSize = 1
	l.writerCfg.maxBackups = 2
	// Compression runs asynchronously in lumberjack and can race TempDir's
	// cleanup. Rotation itself is what this test verifies.
	l.writerCfg.compress = false
	l.mu.Lock()
	for i := 0; i < 4; i++ {
		l.writeLineLocked(strings.Repeat("x", 350*1024))
	}
	l.mu.Unlock()
	if err := l.close(); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "hy2_access-*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one rotated access log")
	}
}
