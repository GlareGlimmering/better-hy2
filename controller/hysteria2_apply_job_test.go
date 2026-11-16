package controller

import (
	"errors"
	"testing"
	"time"
)

func TestHysteria2ApplyJobsReportPendingFailureAndSuccess(t *testing.T) {
	var jobs hysteria2ApplyJobs
	gate := make(chan struct{})
	result, err := jobs.begin(func() error {
		<-gate
		return errors.New("previous configuration restored")
	})
	if err != nil {
		close(gate)
		t.Fatal(err)
	}
	pending, ok := jobs.status(result.ID)
	_, concurrentErr := jobs.begin(func() error { return nil })
	close(gate)
	if !ok || pending.State != "pending" || concurrentErr == nil {
		t.Fatalf("expected one pending job and duplicate protection, got %+v, %v", pending, concurrentErr)
	}
	wait := func(id string) hysteria2ApplyResult {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if status, ok := jobs.status(id); ok && status.State != "pending" {
				return status
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("configuration result remained pending")
		return hysteria2ApplyResult{}
	}
	failed := wait(result.ID)
	if failed.State != "failed" || failed.Error != "previous configuration restored" {
		t.Fatalf("apply failure was not preserved: %+v", failed)
	}
	next, err := jobs.begin(func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if passed := wait(next.ID); passed.State != "succeeded" || passed.Error != "" {
		t.Fatalf("unexpected successful apply result: %+v", passed)
	}
	if previous, ok := jobs.status(result.ID); !ok || previous != failed {
		t.Fatalf("earlier result was overwritten: %+v", previous)
	}
}
