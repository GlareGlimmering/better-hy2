package service

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"h-ui/model/constant"
	"h-ui/util"
)

const (
	LogTypeSystem    = "system"
	LogTypeHysteria2 = "hysteria2"
	LogTypeAccess    = "access"
	LogTypeAll       = "all"
	maxStreamLines   = 200
	tailReadBlock    = 32 * 1024
)

type logSpec struct {
	Type string
	Path string
}

var logSpecs = []logSpec{
	{Type: LogTypeSystem, Path: constant.SystemLogPath},
	{Type: LogTypeHysteria2, Path: constant.Hysteria2LogPath},
	{Type: LogTypeAccess, Path: constant.Hysteria2AccessLogPath},
}

type logCursor struct {
	Offset    int64  `json:"offset"`
	Signature string `json:"signature"`
}

type LogStreamResult struct {
	Lines  []string `json:"lines"`
	Cursor string   `json:"cursor"`
	Reset  bool     `json:"reset"`
	Exists bool     `json:"exists"`
}

type LogUsage struct {
	Type   string `json:"type"`
	Bytes  int64  `json:"bytes"`
	Human  string `json:"human"`
	Files  int    `json:"files"`
	Exists bool   `json:"exists"`
	Error  string `json:"error,omitempty"`
}

type LogUsageSummary struct {
	Logs     []LogUsage `json:"logs"`
	Bytes    int64      `json:"bytes"`
	Human    string     `json:"human"`
	Complete bool       `json:"complete"`
}

func resolveLogSpec(logType string) (logSpec, error) {
	for _, spec := range logSpecs {
		if spec.Type == logType {
			return spec, nil
		}
	}
	return logSpec{}, fmt.Errorf("unsupported log type %q", logType)
}

func ReadLogStream(logType, cursor string, limit int) (LogStreamResult, error) {
	spec, err := resolveLogSpec(logType)
	if err != nil {
		return LogStreamResult{}, err
	}
	return readLogStream(spec, cursor, limit)
}

func readLogStream(spec logSpec, cursor string, limit int) (LogStreamResult, error) {
	if limit <= 0 || limit > maxStreamLines {
		limit = maxStreamLines
	}
	unlock := util.LockLogFileRead(spec.Path)
	defer unlock()

	file, err := os.Open(spec.Path)
	if errors.Is(err, os.ErrNotExist) {
		return LogStreamResult{Lines: []string{}, Exists: false}, nil
	}
	if err != nil {
		return LogStreamResult{}, fmt.Errorf("open %s log: %w", spec.Type, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return LogStreamResult{}, fmt.Errorf("stat %s log: %w", spec.Type, err)
	}
	if !info.Mode().IsRegular() {
		return LogStreamResult{}, fmt.Errorf("%s log is not a regular file", spec.Type)
	}
	signature, err := logFileSignature(file, info.Size())
	if err != nil {
		return LogStreamResult{}, err
	}
	generation := signature
	if identity := logFileIdentity(info); identity != "" {
		generation = identity + ":" + signature
	}

	var state logCursor
	initial := cursor == ""
	reset := false
	if !initial {
		state, err = decodeLogCursor(cursor)
		if err != nil {
			return LogStreamResult{}, err
		}
		if state.Offset > info.Size() || state.Signature != generation {
			reset = true
		}
	}

	var lines []string
	var nextOffset int64
	if initial || reset {
		lines, nextOffset, err = readLastCompleteLines(file, info.Size(), limit)
	} else {
		lines, nextOffset, err = readCompleteLinesFrom(file, state.Offset, limit)
	}
	if err != nil {
		return LogStreamResult{}, fmt.Errorf("read %s log: %w", spec.Type, err)
	}
	nextCursor, err := encodeLogCursor(logCursor{Offset: nextOffset, Signature: generation})
	if err != nil {
		return LogStreamResult{}, err
	}
	return LogStreamResult{
		Lines:  lines,
		Cursor: nextCursor,
		Reset:  reset,
		Exists: true,
	}, nil
}

func logFileIdentity(info os.FileInfo) string {
	value := reflect.ValueOf(info.Sys())
	if !value.IsValid() {
		return ""
	}
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	device := value.FieldByName("Dev")
	inode := value.FieldByName("Ino")
	if !device.IsValid() || !inode.IsValid() || !device.CanUint() || !inode.CanUint() {
		return ""
	}
	return fmt.Sprintf("%d:%d", device.Uint(), inode.Uint())
}

func logFileSignature(file *os.File, size int64) (string, error) {
	if size == 0 {
		return "", nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	reader := bufio.NewReader(io.LimitReader(file, 64*1024))
	line, err := reader.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	sum := sha256.Sum256(line)
	return hex.EncodeToString(sum[:]), nil
}

func readCompleteLinesFrom(file *os.File, offset int64, limit int) ([]string, int64, error) {
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}
	reader := bufio.NewReader(file)
	lines := make([]string, 0, limit)
	next := offset
	for len(lines) < limit {
		line, err := reader.ReadString('\n')
		if len(line) > 0 && strings.HasSuffix(line, "\n") {
			next += int64(len(line))
			lines = append(lines, strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"))
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, next, err
		}
	}
	return lines, next, nil
}

func readLastCompleteLines(file *os.File, size int64, limit int) ([]string, int64, error) {
	if size == 0 {
		return []string{}, 0, nil
	}
	completeEnd, err := lastCompleteLineOffset(file, size)
	if err != nil || completeEnd == 0 {
		return []string{}, completeEnd, err
	}
	position := completeEnd
	data := make([]byte, 0, tailReadBlock)
	for position > 0 && bytes.Count(data, []byte{'\n'}) <= limit {
		start := position - tailReadBlock
		if start < 0 {
			start = 0
		}
		chunk := make([]byte, position-start)
		if _, err = file.ReadAt(chunk, start); err != nil && !errors.Is(err, io.EOF) {
			return nil, 0, err
		}
		data = append(chunk, data...)
		position = start
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return []string{}, completeEnd, nil
	}
	lines := strings.Split(text, "\n")
	if position > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines, completeEnd, nil
}

func lastCompleteLineOffset(file *os.File, size int64) (int64, error) {
	buffer := make([]byte, 1)
	for position := size - 1; position >= 0; position-- {
		if _, err := file.ReadAt(buffer, position); err != nil {
			return 0, err
		}
		if buffer[0] == '\n' {
			return position + 1, nil
		}
	}
	return 0, nil
}

func encodeLogCursor(cursor logCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeLogCursor(value string) (logCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return logCursor{}, errors.New("invalid log cursor")
	}
	var cursor logCursor
	if err = json.Unmarshal(data, &cursor); err != nil || cursor.Offset < 0 {
		return logCursor{}, errors.New("invalid log cursor")
	}
	return cursor, nil
}

func GetLogUsage() LogUsageSummary {
	summary := LogUsageSummary{Logs: make([]LogUsage, 0, len(logSpecs)), Complete: true}
	for _, spec := range logSpecs {
		usage, err := measureLogUsage(spec)
		if err != nil {
			usage.Error = err.Error()
			summary.Complete = false
		}
		summary.Logs = append(summary.Logs, usage)
		summary.Bytes += usage.Bytes
	}
	summary.Human = formatLogBytes(summary.Bytes)
	return summary
}

func measureLogUsage(spec logSpec) (LogUsage, error) {
	usage := LogUsage{Type: spec.Type}
	unlock := util.LockLogFileRead(spec.Path)
	defer unlock()
	files, err := matchingLogFiles(spec)
	if err != nil {
		return usage, fmt.Errorf("inspect %s logs: %w", spec.Type, err)
	}
	for _, path := range files {
		info, statErr := os.Lstat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return usage, statErr
		}
		if !info.Mode().IsRegular() {
			return usage, fmt.Errorf("%s is not a regular file", filepath.Base(path))
		}
		usage.Bytes += info.Size()
		usage.Files++
	}
	usage.Exists = usage.Files > 0
	usage.Human = formatLogBytes(usage.Bytes)
	return usage, nil
}

func ClearLogs(logType string) (LogUsageSummary, error) {
	var specs []logSpec
	if logType == LogTypeAll {
		specs = logSpecs
	} else {
		spec, err := resolveLogSpec(logType)
		if err != nil {
			return LogUsageSummary{}, err
		}
		specs = []logSpec{spec}
	}
	for _, spec := range specs {
		if err := clearLog(spec); err != nil {
			return LogUsageSummary{}, err
		}
	}
	return GetLogUsage(), nil
}

func clearLog(spec logSpec) error {
	unlock := util.LockLogFileWrite(spec.Path)
	defer unlock()
	if err := truncateActiveLog(spec); err != nil {
		return fmt.Errorf("clear %s log: %w", spec.Type, err)
	}
	// Repeat enumeration so a compressor already finishing a rotation cannot
	// leave a newly-created archive behind.
	for pass := 0; pass < 3; pass++ {
		files, err := matchingLogFiles(spec)
		if err != nil {
			return fmt.Errorf("clear %s log history: %w", spec.Type, err)
		}
		removed := false
		for _, path := range files {
			if filepath.Clean(path) == filepath.Clean(spec.Path) {
				continue
			}
			if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove %s: %w", filepath.Base(path), err)
			}
			removed = true
		}
		if !removed {
			break
		}
	}
	return nil
}

func truncateActiveLog(spec logSpec) error {
	info, err := os.Lstat(spec.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("active log is not a regular file")
	}
	file, err := os.OpenFile(spec.Path, os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	return file.Close()
}

func matchingLogFiles(spec logSpec) ([]string, error) {
	dir := filepath.Dir(spec.Path)
	dirInfo, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !dirInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	active := filepath.Base(spec.Path)
	ext := filepath.Ext(active)
	stem := strings.TrimSuffix(active, ext)
	rotated := regexp.MustCompile(`^` + regexp.QuoteMeta(stem) + `-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}(?:\.\d{3})?` + regexp.QuoteMeta(ext) + `(?:\.gz)?$`)
	paths := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == active || rotated.MatchString(name) {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	return paths, nil
}

func formatLogBytes(size int64) string {
	const unit = int64(1024)
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KB", "MB", "GB"}
	for _, name := range units {
		value /= 1024
		if value < 1024 || name == "GB" {
			return fmt.Sprintf("%.2f %s", value, name)
		}
	}
	return fmt.Sprintf("%d B", size)
}
