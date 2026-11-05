package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"h-ui/model/constant"
	"h-ui/util"
)

const (
	accessLogMaxSizeMB  = 50
	accessLogMaxBackups = 7
	accessLogMaxAgeDays = 30
)

type hysteriaEvent struct {
	Message   string  `json:"msg"`
	Addr      string  `json:"addr"`
	ID        string  `json:"id"`
	ReqAddr   string  `json:"reqAddr"`
	SessionID *uint32 `json:"sessionID"`
	Error     string  `json:"error"`
}

type udpSessionKey struct {
	addr      string
	id        string
	sessionID uint32
}

type rotatingWriterConfig struct {
	maxSize    int
	maxBackups int
	maxAge     int
	compress   bool
}

type accessLogger struct {
	mu          sync.Mutex
	path        string
	now         func() time.Time
	writer      io.WriteCloser
	writerCfg   rotatingWriterConfig
	udpTargets  map[udpSessionKey]string
	warnedWrite bool
}

var hysteria2AccessLogger = newAccessLogger(constant.Hysteria2AccessLogPath)

func newAccessLogger(path string) *accessLogger {
	return &accessLogger{
		path: path,
		now:  time.Now,
		writerCfg: rotatingWriterConfig{
			maxSize:    accessLogMaxSizeMB,
			maxBackups: accessLogMaxBackups,
			maxAge:     accessLogMaxAgeDays,
			compress:   true,
		},
		udpTargets: make(map[udpSessionKey]string),
	}
}

func (l *accessLogger) initialize() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.writer != nil {
		return nil
	}
	return l.openLocked()
}

// consume accepts only Hysteria's explicit proxy request EventLogger records.
// QUIC connection lifecycle and other debug records are deliberately ignored.
func (l *accessLogger) consume(raw string) {
	event, ok := parseHysteriaEvent(raw, 0)
	if !ok {
		return
	}
	if event.Addr == "" || event.ID == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	protocol := ""
	result := ""
	target := event.ReqAddr
	switch event.Message {
	case "TCP request":
		protocol, result = "tcp", "requested"
	case "TCP error":
		protocol, result = "tcp", "error"
	case "TCP closed":
		protocol, result = "tcp", "closed"
	case "UDP request":
		if event.SessionID == nil || target == "" {
			return
		}
		protocol, result = "udp", "requested"
		l.udpTargets[udpSessionKey{event.Addr, event.ID, *event.SessionID}] = target
	case "UDP error", "UDP closed":
		if event.SessionID == nil {
			return
		}
		key := udpSessionKey{event.Addr, event.ID, *event.SessionID}
		target = l.udpTargets[key]
		delete(l.udpTargets, key)
		if target == "" {
			return
		}
		protocol = "udp"
		if event.Message == "UDP error" {
			result = "error"
		} else {
			result = "closed"
		}
	default:
		return
	}
	if target == "" {
		return
	}

	line := fmt.Sprintf("%s from %s %s %s:%s [HY2_IN -> DIRECT] user: %s",
		l.now().Format("2006/01/02 15:04:05.000000"),
		sanitizeAccessField(event.Addr), result, protocol,
		sanitizeAccessField(target), sanitizeAccessField(event.ID))
	if event.Error != "" && result == "error" {
		line += " error: " + sanitizeAccessField(event.Error)
	}
	l.writeLineLocked(line)
}

func parseHysteriaEvent(raw string, depth int) (hysteriaEvent, bool) {
	if depth > 1 {
		return hysteriaEvent{}, false
	}

	var event hysteriaEvent
	if err := json.Unmarshal([]byte(raw), &event); err == nil {
		if isAccessEventMessage(event.Message) && event.Addr != "" && event.ID != "" {
			return event, true
		}
		// H-UI wraps each raw core line in its own JSON log entry. Supporting
		// that representation also makes copied production log lines useful as
		// regression fixtures, while the live reader normally sees the inner line.
		if event.Message != "" {
			if nested, ok := parseHysteriaEvent(event.Message, depth+1); ok {
				return nested, true
			}
		}
	}

	// Hysteria's official console encoder writes tab-separated metadata and a
	// structured JSON field object. Parse only the six known EventLogger names;
	// ordinary QUIC/debug messages are never interpreted as access requests.
	parts := strings.Split(raw, "\t")
	for i, part := range parts {
		message := strings.TrimSpace(part)
		if !isAccessEventMessage(message) || i+1 >= len(parts) {
			continue
		}
		var fields hysteriaEvent
		if err := json.Unmarshal([]byte(strings.Join(parts[i+1:], "\t")), &fields); err != nil {
			return hysteriaEvent{}, false
		}
		fields.Message = message
		if fields.Addr == "" || fields.ID == "" {
			return hysteriaEvent{}, false
		}
		return fields, true
	}
	return hysteriaEvent{}, false
}

func isAccessEventMessage(message string) bool {
	switch message {
	case "TCP request", "TCP error", "TCP closed", "UDP request", "UDP error", "UDP closed":
		return true
	default:
		return false
	}
}

func sanitizeAccessField(value string) string {
	return strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(value)
}

func (l *accessLogger) writeLineLocked(line string) {
	if l.writer == nil {
		if err := l.openLocked(); err != nil {
			l.warnWriteFailure(err)
			return
		}
	}
	if _, err := io.WriteString(l.writer, line+"\n"); err != nil {
		l.warnWriteFailure(err)
	}
}

func (l *accessLogger) openLocked() error {
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create access log directory: %w", err)
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("create access log file: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("close access log file: %w", err)
	}
	if err = os.Chmod(l.path, 0640); err != nil {
		return fmt.Errorf("set access log permissions: %w", err)
	}
	l.writer = util.NewSynchronizedWriteCloser(l.path, &lumberjack.Logger{
		Filename:   l.path,
		MaxSize:    l.writerCfg.maxSize,
		MaxBackups: l.writerCfg.maxBackups,
		MaxAge:     l.writerCfg.maxAge,
		Compress:   l.writerCfg.compress,
		LocalTime:  true,
	})
	return nil
}

func (l *accessLogger) warnWriteFailure(err error) {
	// Avoid flooding the service log while keeping proxying independent from
	// access-log storage failures.
	if !l.warnedWrite {
		l.warnedWrite = true
		logrus.Warnf("hysteria2 access log write failed: %v", err)
	}
}

func (l *accessLogger) close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.writer == nil {
		return nil
	}
	err := l.writer.Close()
	l.writer = nil
	return err
}
