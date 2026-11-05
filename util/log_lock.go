package util

import (
	"io"
	"path/filepath"
	"sync"
)

var logFileLocks sync.Map

func logFileLock(path string) *sync.RWMutex {
	key := filepath.Clean(path)
	lock, _ := logFileLocks.LoadOrStore(key, &sync.RWMutex{})
	return lock.(*sync.RWMutex)
}

// LockLogFileRead prevents a management operation from truncating a log while
// a logger is writing or rotating it. The returned function releases the lock.
func LockLogFileRead(path string) func() {
	lock := logFileLock(path)
	lock.RLock()
	return lock.RUnlock
}

// LockLogFileWrite serializes destructive management operations with writers.
func LockLogFileWrite(path string) func() {
	lock := logFileLock(path)
	lock.Lock()
	return lock.Unlock
}

type synchronizedWriter struct {
	path   string
	writer io.Writer
}

func NewSynchronizedWriter(path string, writer io.Writer) io.Writer {
	return &synchronizedWriter{path: path, writer: writer}
}

func (w *synchronizedWriter) Write(p []byte) (int, error) {
	unlock := LockLogFileRead(w.path)
	defer unlock()
	return w.writer.Write(p)
}

type synchronizedWriteCloser struct {
	path   string
	writer io.WriteCloser
}

func NewSynchronizedWriteCloser(path string, writer io.WriteCloser) io.WriteCloser {
	return &synchronizedWriteCloser{path: path, writer: writer}
}

func (w *synchronizedWriteCloser) Write(p []byte) (int, error) {
	unlock := LockLogFileRead(w.path)
	defer unlock()
	return w.writer.Write(p)
}

func (w *synchronizedWriteCloser) Close() error {
	unlock := LockLogFileWrite(w.path)
	defer unlock()
	return w.writer.Close()
}
