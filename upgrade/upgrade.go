package upgrade

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"h-ui/model/constant"
)

const (
	DefaultRepository   = "GlareGlimmering/h-ui"
	BinaryAsset         = "h-ui-linux-amd64"
	ChecksumAsset       = "SHA256SUMS"
	DefaultPanelPort    = constant.PanelPort
	DefaultUnitPath     = "/etc/systemd/system/h-ui.service"
	panelHealthAttempts = 60
	panelHealthInterval = time.Second
)

var releaseTagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+-custom\.\d+(?:-rc\.\d+)?$`)
var scriptPattern = regexp.MustCompile(`<script[^>]+src=["']([^"']+)["']`)

type Options struct {
	Version         string
	Prerelease      bool
	Yes             bool
	CurrentVersion  string
	Repository      string
	APIBase         string
	InstallDir      string
	DataDir         string
	StateDir        string
	ServiceName     string
	SystemdUnitPath string
	HealthURL       string
	BackupLimit     int
	Input           io.Reader
	Output          io.Writer
}

type Dependencies struct {
	GOOS           string
	GOARCH         string
	EUID           int
	HTTPClient     *http.Client
	Command        func(string, ...string) ([]byte, error)
	BackupDatabase func(string, string) error
	Now            func() time.Time
	Sleep          func(time.Duration)
}

type releaseInfo struct {
	TagName    string         `json:"tag_name"`
	Prerelease bool           `json:"prerelease"`
	Draft      bool           `json:"draft"`
	Assets     []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type snapshot struct {
	Dir      string
	Files    map[string]string
	Metadata map[string]fileMetadata
	Absent   []string
}

type snapshotManifest struct {
	Files    map[string]string       `json:"files"`
	Metadata map[string]fileMetadata `json:"metadata"`
	Absent   []string                `json:"absent"`
}

type fileMetadata struct {
	Mode uint32 `json:"mode"`
	UID  int    `json:"uid"`
	GID  int    `json:"gid"`
}

func DefaultOptions() Options {
	return Options{
		Version: "latest", Repository: DefaultRepository,
		APIBase:    "https://api.github.com",
		InstallDir: "/usr/local/h-ui", DataDir: "/usr/local/h-ui",
		StateDir: "/var/lib/h-ui", ServiceName: "h-ui.service", SystemdUnitPath: DefaultUnitPath,
		BackupLimit: 5, Input: os.Stdin, Output: os.Stdout,
	}
}

func DefaultDependencies() Dependencies {
	return Dependencies{
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, EUID: currentEUID(),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Command: func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).CombinedOutput()
		},
		BackupDatabase: backupSQLiteDatabase,
		Now:            time.Now,
		Sleep:          time.Sleep,
	}
}

func Run(ctx context.Context, opts Options, deps Dependencies) error {
	if err := normalizeAndValidate(&opts, &deps); err != nil {
		return err
	}
	release, err := resolveRelease(ctx, opts, deps)
	if err != nil {
		return fmt.Errorf("resolve target release: %w", err)
	}
	if !releaseTagPattern.MatchString(release.TagName) {
		return fmt.Errorf("release tag %q is not an allowed h-ui version", release.TagName)
	}
	if opts.Version == "latest" {
		isRCTag := strings.Contains(release.TagName, "-rc.")
		if !opts.Prerelease && (release.Prerelease || isRCTag) {
			return errors.New("stable channel returned a prerelease; refusing upgrade")
		}
		if opts.Prerelease && (!release.Prerelease || !isRCTag) {
			return errors.New("prerelease channel returned a stable release; refusing upgrade")
		}
		if releaseTagPattern.MatchString(opts.CurrentVersion) && compareReleaseTags(release.TagName, opts.CurrentVersion) < 0 {
			return fmt.Errorf("no upgrade available: current %s is newer than automatic channel target %s; refusing automatic downgrade", opts.CurrentVersion, release.TagName)
		}
	}
	if opts.CurrentVersion == release.TagName {
		fmt.Fprintf(opts.Output, "h-ui is already at %s\n", release.TagName)
		return nil
	}

	workDir, err := os.MkdirTemp(opts.InstallDir, ".upgrade-download-*")
	if err != nil {
		return fmt.Errorf("create upgrade staging directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	candidate := filepath.Join(workDir, BinaryAsset)
	checksumPath := filepath.Join(workDir, ChecksumAsset)
	if err := downloadReleaseAssets(ctx, release, candidate, checksumPath, deps); err != nil {
		return err
	}
	if err := verifyCandidate(candidate, checksumPath, release.TagName, deps); err != nil {
		return err
	}

	channel := "stable"
	if release.Prerelease {
		channel = "prerelease"
	}
	fmt.Fprintf(opts.Output, "Current: %s\nTarget: %s\nChannel: %s\n", opts.CurrentVersion, release.TagName, channel)
	if !opts.Yes {
		fmt.Fprint(opts.Output, "Continue with the upgrade? [y/N]: ")
		answer, _ := bufio.NewReader(opts.Input).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			return errors.New("upgrade cancelled")
		}
	}

	active, err := serviceState(opts.ServiceName, deps)
	if err != nil {
		return fmt.Errorf("inspect h-ui service state: %w", err)
	}
	snap, err := createSnapshot(opts, release.TagName, deps)
	if err != nil {
		return fmt.Errorf("create pre-upgrade snapshot: %w", err)
	}
	if err := activateCandidate(candidate, release.TagName, active, snap, opts, deps); err != nil {
		return err
	}
	if err := writeUpgradeRecord(snap.Dir, opts.CurrentVersion, release.TagName, "success", deps.Now()); err != nil {
		fmt.Fprintf(opts.Output, "Warning: could not write upgrade record: %v\n", err)
	}
	pruneBackups(filepath.Join(opts.StateDir, "backups"), opts.BackupLimit)
	if active {
		fmt.Fprintf(opts.Output, "Upgrade successful. Running version: %s\n", release.TagName)
	} else {
		fmt.Fprintf(opts.Output, "Upgrade installed %s; h-ui.service remains stopped because it was stopped before the upgrade.\n", release.TagName)
	}
	return nil
}

func normalizeAndValidate(opts *Options, deps *Dependencies) error {
	defaults := DefaultOptions()
	if opts.Version == "" {
		opts.Version = defaults.Version
	}
	if opts.Repository == "" {
		opts.Repository = defaults.Repository
	}
	if opts.APIBase == "" {
		opts.APIBase = defaults.APIBase
	}
	if opts.InstallDir == "" {
		opts.InstallDir = defaults.InstallDir
	}
	if opts.DataDir == "" {
		opts.DataDir = defaults.DataDir
	}
	if opts.StateDir == "" {
		opts.StateDir = defaults.StateDir
	}
	if opts.ServiceName == "" {
		opts.ServiceName = defaults.ServiceName
	}
	if opts.SystemdUnitPath == "" {
		opts.SystemdUnitPath = defaults.SystemdUnitPath
	}
	if opts.BackupLimit == 0 {
		opts.BackupLimit = defaults.BackupLimit
	}
	if opts.Input == nil {
		opts.Input = defaults.Input
	}
	if opts.Output == nil {
		opts.Output = defaults.Output
	}
	if deps.HTTPClient == nil {
		deps.HTTPClient = DefaultDependencies().HTTPClient
	}
	if deps.Command == nil {
		deps.Command = DefaultDependencies().Command
	}
	if deps.BackupDatabase == nil {
		deps.BackupDatabase = backupSQLiteDatabase
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Sleep == nil {
		deps.Sleep = time.Sleep
	}
	if deps.GOOS == "" {
		deps.GOOS = runtime.GOOS
	}
	if deps.GOARCH == "" {
		deps.GOARCH = runtime.GOARCH
	}
	if deps.GOOS != "linux" || deps.GOARCH != "amd64" {
		return fmt.Errorf("unsupported platform %s/%s; D10 supports linux/amd64", deps.GOOS, deps.GOARCH)
	}
	if deps.EUID != 0 {
		return errors.New("h-ui upgrade must be run as root")
	}
	if opts.Version != "latest" && !releaseTagPattern.MatchString(opts.Version) {
		return fmt.Errorf("invalid target version %q", opts.Version)
	}
	if _, err := os.Stat(filepath.Join(opts.InstallDir, "h-ui")); err != nil {
		return fmt.Errorf("existing h-ui installation not found: %w", err)
	}
	return nil
}

func resolveRelease(ctx context.Context, opts Options, deps Dependencies) (releaseInfo, error) {
	base := strings.TrimRight(opts.APIBase, "/") + "/repos/" + opts.Repository + "/releases"
	if opts.Version != "latest" {
		return fetchRelease(ctx, base+"/tags/"+url.PathEscape(opts.Version), deps)
	}
	if !opts.Prerelease {
		return fetchRelease(ctx, base+"/latest", deps)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"?per_page=30", nil)
	req.Header.Set("User-Agent", "h-ui-upgrader")
	resp, err := deps.HTTPClient.Do(req)
	if err != nil {
		return releaseInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return releaseInfo{}, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	var releases []releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return releaseInfo{}, err
	}
	var selected *releaseInfo
	for index := range releases {
		release := &releases[index]
		if release.Draft || !release.Prerelease || !strings.Contains(release.TagName, "-rc.") || !releaseTagPattern.MatchString(release.TagName) {
			continue
		}
		if selected == nil || compareReleaseTags(release.TagName, selected.TagName) > 0 {
			selected = release
		}
	}
	if selected != nil {
		return *selected, nil
	}
	return releaseInfo{}, errors.New("no allowed prerelease found")
}

func compareReleaseTags(left, right string) int {
	leftParts, leftOK := releaseTagParts(left)
	rightParts, rightOK := releaseTagParts(right)
	if !leftOK || !rightOK {
		return strings.Compare(left, right)
	}
	for index := range leftParts {
		if leftParts[index] < rightParts[index] {
			return -1
		}
		if leftParts[index] > rightParts[index] {
			return 1
		}
	}
	return 0
}

func releaseTagParts(tag string) ([6]int, bool) {
	match := releaseTagPattern.FindStringSubmatch(tag)
	if match == nil {
		return [6]int{}, false
	}
	// The release regex validates the full shape; split the numeric fields here
	// so rc.10 is compared numerically rather than lexicographically with rc.9.
	numeric := regexp.MustCompile(`\d+`).FindAllString(tag, -1)
	if len(numeric) < 4 || len(numeric) > 5 {
		return [6]int{}, false
	}
	parts := [6]int{}
	for index, value := range numeric[:4] {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return [6]int{}, false
		}
		parts[index] = parsed
	}
	if len(numeric) == 5 {
		parts[4] = 0 // prerelease sorts before the matching stable release
		parts[5], _ = strconv.Atoi(numeric[4])
	} else {
		parts[4] = 1
	}
	return parts, true
}

func fetchRelease(ctx context.Context, endpoint string, deps Dependencies) (releaseInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("User-Agent", "h-ui-upgrader")
	resp, err := deps.HTTPClient.Do(req)
	if err != nil {
		return releaseInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return releaseInfo{}, fmt.Errorf("release not found (HTTP %d)", resp.StatusCode)
	}
	var release releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return releaseInfo{}, err
	}
	if release.Draft {
		return releaseInfo{}, errors.New("draft releases cannot be installed")
	}
	return release, nil
}

func downloadReleaseAssets(ctx context.Context, release releaseInfo, binary, sums string, deps Dependencies) error {
	assets := map[string]string{}
	for _, asset := range release.Assets {
		assets[asset.Name] = asset.URL
	}
	for name, destination := range map[string]string{BinaryAsset: binary, ChecksumAsset: sums} {
		assetURL := assets[name]
		if assetURL == "" {
			return fmt.Errorf("release %s is missing %s", release.TagName, name)
		}
		if err := downloadFile(ctx, assetURL, destination, deps); err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
	}
	return nil
}

func downloadFile(ctx context.Context, source, destination string, deps Dependencies) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	req.Header.Set("User-Agent", "h-ui-upgrader")
	resp, err := deps.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, resp.Body)
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func verifyCandidate(candidate, checksumPath, targetVersion string, deps Dependencies) error {
	expected, err := expectedChecksum(checksumPath, BinaryAsset)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(candidate)
	if err != nil {
		return err
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(content))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("SHA-256 mismatch: got %s, expected %s", actual, expected)
	}
	// Permissions are preflight: a missing executable bit must never be found
	// after the production service has already been stopped.
	if err := os.Chmod(candidate, 0755); err != nil {
		return fmt.Errorf("make candidate executable: %w", err)
	}
	output, err := deps.Command(candidate, "version")
	if err != nil {
		return fmt.Errorf("execute candidate version check: %w: %s", err, strings.TrimSpace(string(output)))
	}
	reported := strings.TrimSpace(string(output))
	if reported != targetVersion && reported != "h-ui version "+targetVersion {
		return fmt.Errorf("candidate reports %q, expected exact version %s", reported, targetVersion)
	}
	return nil
}

func expectedChecksum(path, asset string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset && regexp.MustCompile(`^[a-fA-F0-9]{64}$`).MatchString(fields[0]) {
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s does not contain a valid checksum for %s", path, asset)
}

func createSnapshot(opts Options, target string, deps Dependencies) (snapshot, error) {
	root := filepath.Join(opts.StateDir, "backups")
	name := "upgrade-" + deps.Now().UTC().Format("20060102T150405.000000000Z") + "-" + strings.TrimPrefix(target, "v")
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return snapshot{}, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(dir)
		}
	}()
	snap := snapshot{Dir: dir, Files: map[string]string{}, Metadata: map[string]fileMetadata{}}
	paths := []string{
		filepath.Join(opts.InstallDir, "h-ui"),
		opts.SystemdUnitPath,
		filepath.Join(opts.DataDir, "bin", "hysteria2.yaml"),
		filepath.Join(opts.StateDir, "xray", "config.json"),
		filepath.Join(opts.StateDir, "xray", "config.previous.json"),
		filepath.Join(opts.StateDir, "xray", "candidate.json"),
		filepath.Join(opts.StateDir, "xray", "certs", "ca.crt"),
	}
	for index, source := range paths {
		info, err := os.Stat(source)
		if err == nil && !info.IsDir() {
			destination := filepath.Join(dir, fmt.Sprintf("file-%d-%s", index, filepath.Base(source)))
			if err := copyFile(source, destination, info.Mode().Perm()); err != nil {
				return snapshot{}, err
			}
			snap.Files[source] = destination
			uid, gid := fileOwnership(info)
			snap.Metadata[source] = fileMetadata{Mode: uint32(info.Mode().Perm()), UID: uid, GID: gid}
		} else if os.IsNotExist(err) {
			snap.Absent = append(snap.Absent, source)
		} else if err != nil {
			return snapshot{}, fmt.Errorf("inspect %s: %w", source, err)
		}
	}
	database := filepath.Join(opts.DataDir, "data", "h_ui.db")
	if _, err := os.Stat(database); err == nil {
		destination := filepath.Join(dir, "h_ui.db")
		if err := deps.BackupDatabase(database, destination); err != nil {
			return snapshot{}, fmt.Errorf("backup SQLite database: %w", err)
		}
		snap.Files[database] = destination
		if info, infoErr := os.Stat(database); infoErr == nil {
			uid, gid := fileOwnership(info)
			snap.Metadata[database] = fileMetadata{Mode: uint32(info.Mode().Perm()), UID: uid, GID: gid}
		}
	} else if os.IsNotExist(err) {
		snap.Absent = append(snap.Absent, database)
	} else {
		return snapshot{}, fmt.Errorf("inspect SQLite database: %w", err)
	}
	manifest, err := json.MarshalIndent(snapshotManifest{Files: snap.Files, Metadata: snap.Metadata, Absent: snap.Absent}, "", "  ")
	if err != nil {
		return snapshot{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0640); err != nil {
		return snapshot{}, err
	}
	if err := writeUpgradeRecord(dir, opts.CurrentVersion, target, "prepared", deps.Now()); err != nil {
		return snapshot{}, err
	}
	complete = true
	return snap, nil
}

func activateCandidate(candidate, target string, wasActive bool, snap snapshot, opts Options, deps Dependencies) error {
	binary := filepath.Join(opts.InstallDir, "h-ui")
	previous := filepath.Join(opts.InstallDir, ".h-ui-upgrade-previous")
	_ = os.Remove(previous)
	if wasActive {
		if output, err := deps.Command("systemctl", "stop", opts.ServiceName); err != nil {
			return fmt.Errorf("stop current h-ui: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	if err := os.Rename(binary, previous); err != nil {
		if wasActive {
			if restoreErr := restartAndVerifyPrevious(opts, deps); restoreErr != nil {
				return fmt.Errorf("preserve current binary: %v; CRITICAL: restart unchanged service failed: %w", err, restoreErr)
			}
		}
		return fmt.Errorf("preserve current binary: %w", err)
	}
	if err := os.Rename(candidate, binary); err != nil {
		if restoreErr := os.Rename(previous, binary); restoreErr != nil {
			return fmt.Errorf("activate new binary: %v; CRITICAL: restore previous binary failed: %w", err, restoreErr)
		}
		if wasActive {
			if restoreErr := restartAndVerifyPrevious(opts, deps); restoreErr != nil {
				return fmt.Errorf("activate new binary: %v; CRITICAL: restart previous service failed: %w", err, restoreErr)
			}
		}
		return fmt.Errorf("activate new binary: %w", err)
	}

	var startOutput []byte
	var startErr error
	if err := writeSystemdUnit(opts); err != nil {
		startErr = fmt.Errorf("write standard systemd unit: %w", err)
	}
	if startErr == nil {
		if output, err := deps.Command("systemctl", "daemon-reload"); err != nil {
			startErr = fmt.Errorf("reload systemd unit: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	if startErr == nil && wasActive {
		startOutput, startErr = deps.Command("systemctl", "start", opts.ServiceName)
		if startErr == nil && !serviceActive(opts.ServiceName, deps) {
			startErr = errors.New("systemd did not report the service active")
		}
		if startErr == nil {
			startErr = waitForHealthyPanel(opts, deps)
		}
	}
	if startErr == nil {
		_ = os.Remove(previous)
		return nil
	}
	applyErr := fmt.Errorf("new version %s failed to start or pass health checks: %w: %s", target, startErr, strings.TrimSpace(string(startOutput)))
	rollbackErr := rollback(previous, binary, wasActive, snap, opts, deps)
	if rollbackErr != nil {
		return fmt.Errorf("upgrade failed: %v; CRITICAL: rollback failed: %w", applyErr, rollbackErr)
	}
	_ = writeUpgradeRecord(snap.Dir, opts.CurrentVersion, target, "rolled-back", deps.Now())
	return fmt.Errorf("upgrade failed: %v; rollback successful; running previous version: %s", applyErr, opts.CurrentVersion)
}

func rollback(previous, binary string, wasActive bool, snap snapshot, opts Options, deps Dependencies) error {
	_, _ = deps.Command("systemctl", "stop", opts.ServiceName)
	failed := binary + ".failed"
	_ = os.Remove(failed)
	if err := os.Rename(binary, failed); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("preserve failed binary: %w", err)
	}
	if err := os.Rename(previous, binary); err != nil {
		backup := snap.Files[binary]
		info, backupErr := os.Stat(backup)
		if backup == "" || backupErr != nil {
			return fmt.Errorf("restore previous binary: %w (snapshot fallback unavailable: %v)", err, backupErr)
		}
		if backupErr = copyFile(backup, binary, info.Mode().Perm()); backupErr != nil {
			return fmt.Errorf("restore previous binary from snapshot: %w", backupErr)
		}
		if metadata, ok := snap.Metadata[binary]; ok {
			if backupErr = restoreFileMetadata(binary, metadata); backupErr != nil {
				return fmt.Errorf("restore previous binary ownership: %w", backupErr)
			}
		}
	}
	for _, path := range snap.Absent {
		if path == binary {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove newly-created file %s during rollback: %w", path, err)
		}
	}
	for destination, source := range snap.Files {
		if destination == binary {
			continue
		}
		info, err := os.Stat(source)
		if err != nil {
			return fmt.Errorf("read snapshot %s: %w", source, err)
		}
		if err := copyFile(source, destination, info.Mode().Perm()); err != nil {
			return fmt.Errorf("restore %s: %w", destination, err)
		}
		if metadata, ok := snap.Metadata[destination]; ok {
			if err := restoreFileMetadata(destination, metadata); err != nil {
				return fmt.Errorf("restore metadata for %s: %w", destination, err)
			}
		}
	}
	if output, err := deps.Command("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("reload restored systemd unit: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if !wasActive {
		_ = os.Remove(failed)
		return nil
	}
	if output, err := deps.Command("systemctl", "start", opts.ServiceName); err != nil {
		return fmt.Errorf("restart previous service: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if !serviceActive(opts.ServiceName, deps) {
		return errors.New("previous service did not become active after rollback")
	}
	if err := waitForPreviousHealthyPanel(opts, deps); err != nil {
		return fmt.Errorf("previous version health check: %w", err)
	}
	_ = os.Remove(failed)
	return nil
}

func serviceActive(name string, deps Dependencies) bool {
	active, err := serviceState(name, deps)
	return err == nil && active
}

func serviceState(name string, deps Dependencies) (bool, error) {
	output, err := deps.Command("systemctl", "is-active", name)
	state := strings.TrimSpace(string(output))
	if err == nil {
		if state != "active" {
			return false, fmt.Errorf("unexpected successful systemd state %q", state)
		}
		return true, nil
	}
	switch state {
	case "inactive", "failed", "deactivating":
		return false, nil
	default:
		return false, fmt.Errorf("systemctl is-active failed: %w: %s", err, state)
	}
}

func restartAndVerifyPrevious(opts Options, deps Dependencies) error {
	if output, err := deps.Command("systemctl", "start", opts.ServiceName); err != nil {
		return fmt.Errorf("start service: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if !serviceActive(opts.ServiceName, deps) {
		return errors.New("service did not become active")
	}
	return waitForPreviousHealthyPanel(opts, deps)
}

func writeSystemdUnit(opts Options) error {
	directory := filepath.Dir(opts.SystemdUnitPath)
	temporary, err := os.CreateTemp(directory, ".h-ui.service.*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	content := fmt.Sprintf(`[Unit]
Description=h-ui Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=%s
Environment=HUI_DATA=%s/
Environment=GIN_MODE=release
ExecStart=%s/h-ui -p %s
Restart=on-failure
RestartSec=3s
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
`, opts.InstallDir, strings.TrimSuffix(opts.DataDir, "/"), opts.InstallDir, DefaultPanelPort)
	if _, err = temporary.WriteString(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Chmod(0644); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, opts.SystemdUnitPath)
}

func waitForHealthyPanel(opts Options, deps Dependencies) error {
	healthURL := opts.HealthURL
	if healthURL == "" {
		healthURL = panelHealthURL(filepath.Join(opts.DataDir, "data", "h_ui.db"), DefaultPanelPort)
	}
	return waitForPanelURL(healthURL, deps)
}

func waitForPreviousHealthyPanel(opts Options, deps Dependencies) error {
	healthURL := opts.HealthURL
	if healthURL == "" {
		healthURL = panelHealthURL(filepath.Join(opts.DataDir, "data", "h_ui.db"), "")
	}
	return waitForPanelURL(healthURL, deps)
}

func waitForPanelURL(healthURL string, deps Dependencies) error {
	var lastErr error
	for attempt := 0; attempt < panelHealthAttempts; attempt++ {
		if err := checkPanelHealth(healthURL); err == nil {
			return nil
		} else {
			lastErr = err
		}
		deps.Sleep(panelHealthInterval)
	}
	return lastErr
}

func panelHealthURL(database, forcedPort string) string {
	port, webContext, scheme := "8081", "/", "http"
	if db, err := sql.Open("sqlite", database); err == nil {
		defer db.Close()
		var value string
		if db.QueryRow(`SELECT value FROM config WHERE key='H_UI_WEB_PORT'`).Scan(&value) == nil && value != "" {
			port = value
		}
		if db.QueryRow(`SELECT value FROM config WHERE key='H_UI_WEB_CONTEXT'`).Scan(&value) == nil && value != "" {
			webContext = value
		}
		var certPath, keyPath string
		_ = db.QueryRow(`SELECT value FROM config WHERE key='H_UI_CRT_PATH'`).Scan(&certPath)
		_ = db.QueryRow(`SELECT value FROM config WHERE key='H_UI_KEY_PATH'`).Scan(&keyPath)
		if certPath != "" && keyPath != "" {
			scheme = "https"
		}
	}
	if forcedPort != "" {
		port = forcedPort
		scheme = "http"
	}
	if !strings.HasPrefix(webContext, "/") {
		webContext = "/" + webContext
	}
	if !strings.HasSuffix(webContext, "/") {
		webContext += "/"
	}
	return fmt.Sprintf("%s://127.0.0.1:%s%s", scheme, port, webContext)
}

func checkPanelHealth(endpoint string) error {
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} // loopback probe only
	resp, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("panel returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if !strings.Contains(string(body), `<div id="app"></div>`) {
		return errors.New("panel HTML app marker is missing")
	}
	match := scriptPattern.FindSubmatch(body)
	if len(match) <= 1 {
		return errors.New("panel HTML frontend entry asset is missing")
	}
	base, _ := url.Parse(endpoint)
	asset, err := base.Parse(string(match[1]))
	if err != nil {
		return err
	}
	assetResp, err := client.Get(asset.String())
	if err != nil {
		return fmt.Errorf("load frontend entry asset: %w", err)
	}
	defer assetResp.Body.Close()
	if assetResp.StatusCode != http.StatusOK {
		return fmt.Errorf("frontend entry asset returned HTTP %d", assetResp.StatusCode)
	}
	return nil
}

func backupSQLiteDatabase(source, destination string) error {
	_ = os.Remove(destination)
	db, err := sql.Open("sqlite", source)
	if err != nil {
		return err
	}
	defer db.Close()
	quoted := strings.ReplaceAll(destination, "'", "''")
	if _, err := db.Exec("VACUUM INTO '" + quoted + "'"); err != nil {
		return err
	}
	check, err := sql.Open("sqlite", destination)
	if err != nil {
		return err
	}
	defer check.Close()
	var result string
	if err := check.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("backup integrity check: %s", result)
	}
	return nil
}

func copyFile(source, destination string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0750); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	temp, err := os.CreateTemp(filepath.Dir(destination), ".restore-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := io.Copy(temp, input); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, destination)
}

func writeUpgradeRecord(dir, current, target, status string, at time.Time) error {
	record := map[string]string{"current": current, "target": target, "status": status, "time": at.UTC().Format(time.RFC3339)}
	content, _ := json.MarshalIndent(record, "", "  ")
	return os.WriteFile(filepath.Join(dir, "upgrade.json"), content, 0640)
}

func pruneBackups(root string, limit int) {
	if limit < 1 {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "upgrade-") {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)
	for len(dirs) > limit {
		candidate := filepath.Clean(filepath.Join(root, dirs[0]))
		if filepath.Dir(candidate) == filepath.Clean(root) {
			_ = os.RemoveAll(candidate)
		}
		dirs = dirs[1:]
	}
}
