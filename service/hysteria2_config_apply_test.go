package service

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"h-ui/model/bo"
)

type fakeHysteria2ApplyState struct {
	db             []byte
	files          map[string][]byte
	running        bool
	startCalls     int
	failPrepare    error
	failCandidate  error
	failInstall    error
	failValidation error
	failNewStart   error
	failRollbackDB error
	failRollbackIO error
}

func (s *fakeHysteria2ApplyState) ops() hysteria2ConfigApplyOps {
	dbWrites := 0
	return hysteria2ConfigApplyOps{
		prepare: func(bo.Hysteria2ServerConfig) ([]byte, error) {
			if s.failPrepare != nil {
				return nil, s.failPrepare
			}
			return []byte("new"), nil
		},
		readDB: func() ([]byte, error) { return append([]byte(nil), s.db...), nil },
		writeDB: func(value []byte) error {
			dbWrites++
			if dbWrites > 1 && s.failRollbackDB != nil {
				return s.failRollbackDB
			}
			s.db = append([]byte(nil), value...)
			return nil
		},
		readFile: func(path string) ([]byte, error) {
			value, ok := s.files[path]
			if !ok {
				return nil, fs.ErrNotExist
			}
			return append([]byte(nil), value...), nil
		},
		writeFile: func(path string, value []byte, _ os.FileMode) error {
			if strings.HasSuffix(path, ".candidate") && s.failCandidate != nil {
				return s.failCandidate
			}
			if path == "hysteria2.yaml" && string(value) == "new" && s.failInstall != nil {
				return s.failInstall
			}
			if path == "hysteria2.yaml" && string(value) == "old-yaml" && s.failRollbackIO != nil {
				return s.failRollbackIO
			}
			s.files[path] = append([]byte(nil), value...)
			return nil
		},
		remove:    func(path string) error { delete(s.files, path); return nil },
		validate:  func(string) error { return s.failValidation },
		isRunning: func() bool { return s.running },
		stop: func() error {
			s.running = false
			delete(s.files, "hysteria2.yaml")
			return nil
		},
		start: func() error {
			s.startCalls++
			if s.startCalls == 1 && s.failNewStart != nil {
				return s.failNewStart
			}
			s.running = true
			return nil
		},
		path: "hysteria2.yaml",
	}
}

func newFakeHysteria2ApplyState() *fakeHysteria2ApplyState {
	return &fakeHysteria2ApplyState{
		db:      []byte("old-db"),
		files:   map[string][]byte{"hysteria2.yaml": []byte("old-yaml")},
		running: true,
	}
}

func TestApplyHysteria2ConfigRejectsInvalidAndWriteFailuresBeforeMutation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		adjust func(*fakeHysteria2ApplyState)
		want   string
	}{
		{"invalid business config", func(s *fakeHysteria2ApplyState) { s.failPrepare = errors.New("invalid outbound") }, "validate new"},
		{"candidate write", func(s *fakeHysteria2ApplyState) { s.failCandidate = errors.New("disk full") }, "write Hysteria2 candidate"},
		{"candidate validation", func(s *fakeHysteria2ApplyState) { s.failValidation = errors.New("bad YAML") }, "validate Hysteria2 candidate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := newFakeHysteria2ApplyState()
			tt.adjust(state)
			err := applyHysteria2Config(bo.Hysteria2ServerConfig{}, state.ops())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v", err)
			}
			if string(state.db) != "old-db" || string(state.files["hysteria2.yaml"]) != "old-yaml" || !state.running {
				t.Fatalf("state mutated: db=%q yaml=%q running=%v", state.db, state.files["hysteria2.yaml"], state.running)
			}
		})
	}
}

func TestApplyHysteria2ConfigRollsBackInstallAndStartFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		adjust func(*fakeHysteria2ApplyState)
	}{
		{"install", func(s *fakeHysteria2ApplyState) { s.failInstall = errors.New("rename failed") }},
		{"start", func(s *fakeHysteria2ApplyState) { s.failNewStart = errors.New("start failed") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := newFakeHysteria2ApplyState()
			tt.adjust(state)
			err := applyHysteria2Config(bo.Hysteria2ServerConfig{}, state.ops())
			if err == nil || !strings.Contains(err.Error(), "previous configuration restored") {
				t.Fatalf("error = %v", err)
			}
			if string(state.db) != "old-db" || string(state.files["hysteria2.yaml"]) != "old-yaml" || !state.running {
				t.Fatalf("rollback state: db=%q yaml=%q running=%v", state.db, state.files["hysteria2.yaml"], state.running)
			}
		})
	}
}

func TestApplyHysteria2ConfigReportsRollbackFailure(t *testing.T) {
	state := newFakeHysteria2ApplyState()
	state.failNewStart = errors.New("new start failed")
	state.failRollbackDB = errors.New("database locked")
	state.failRollbackIO = errors.New("read only filesystem")
	err := applyHysteria2Config(bo.Hysteria2ServerConfig{}, state.ops())
	if err == nil || !strings.Contains(err.Error(), "rollback failed") || !strings.Contains(err.Error(), "database locked") || !strings.Contains(err.Error(), "read only filesystem") {
		t.Fatalf("error = %v", err)
	}
}

func TestApplyHysteria2ConfigSerializesConcurrentUpdates(t *testing.T) {
	var active int32
	var maxActive int32
	makeOps := func() hysteria2ConfigApplyOps {
		state := newFakeHysteria2ApplyState()
		ops := state.ops()
		ops.prepare = func(bo.Hysteria2ServerConfig) ([]byte, error) {
			current := atomic.AddInt32(&active, 1)
			for {
				maximum := atomic.LoadInt32(&maxActive)
				if current <= maximum || atomic.CompareAndSwapInt32(&maxActive, maximum, current) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			atomic.AddInt32(&active, -1)
			return []byte("new"), nil
		}
		return ops
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := applyHysteria2ConfigSerialized(bo.Hysteria2ServerConfig{}, makeOps()); err != nil {
				t.Errorf("apply error = %v", err)
			}
		}()
	}
	wg.Wait()
	if maxActive != 1 {
		t.Fatalf("maximum concurrent applies = %d, want 1", maxActive)
	}
}
