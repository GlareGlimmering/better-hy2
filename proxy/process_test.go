package proxy

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProcessReconcilesExitedChildAndCanRestart(t *testing.T) {
	child := &process{mutex: &sync.Mutex{}}
	err := child.start("/bin/sh", "-c", "exit 23")
	if err == nil || !strings.Contains(err.Error(), "process exited during startup") {
		t.Fatalf("early process exit was not reported: %v", err)
	}
	if child.isRunning() {
		t.Fatal("exited child remained marked running")
	}

	if err = child.start("/bin/sh", "-c", "sleep 5"); err != nil {
		t.Fatalf("restart after an exited child failed: %v", err)
	}
	if !child.isRunning() {
		t.Fatal("live child was not marked running")
	}
	if err = child.stop(); err != nil {
		t.Fatalf("stop live child: %v", err)
	}
	if child.isRunning() {
		t.Fatal("stopped child remained marked running")
	}
}

func TestProcessStatusReapsChildThatExitsAfterStartup(t *testing.T) {
	child := &process{mutex: &sync.Mutex{}}
	if err := child.start("/bin/sh", "-c", "sleep 0.25; exit 7"); err != nil {
		t.Fatalf("start short-lived child: %v", err)
	}
	if !child.isRunning() {
		t.Fatal("child exited before the initial running check")
	}
	time.Sleep(250 * time.Millisecond)
	if child.isRunning() {
		t.Fatal("asynchronously exited child remained marked running")
	}
}

func TestProcessStartupErrorIncludesPrimaryStderr(t *testing.T) {
	child := &process{mutex: &sync.Mutex{}}
	err := child.start("/bin/sh", "-c", "echo 'listen udp :443: bind: address already in use' >&2; exit 1")
	if err == nil || !strings.Contains(err.Error(), "listen udp :443: bind: address already in use") {
		t.Fatalf("startup error=%v", err)
	}
}
