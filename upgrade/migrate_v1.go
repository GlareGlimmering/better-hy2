package upgrade

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/proxy"
	"h-ui/util"
)

const (
	migrationSource    = "v1.0.0"
	migrationPanelSHA  = "2323bf6d838a48f56d3c77a1b45bf7926be1ec04db0c905352b65bed24f2bd0f"
	migrationKernelSHA = "f1577bff290f5591a651b88596103045390581a6c0a6e999611cab8baa653d9e"
)

func MigrateV1(ctx context.Context, assets string, opts Options, deps Dependencies) error {
	if err := normalizeAndValidate(&opts, &deps); err != nil {
		return err
	}
	if !usesPairedStableLine(opts.Version) {
		return errors.New("migration requires an explicit v2 release")
	}
	opts.CurrentVersion = migrationSource
	binary := filepath.Join(opts.InstallDir, "h-ui")
	kernel := filepath.Join(opts.DataDir, util.GetHysteria2BinPath())
	database := filepath.Join(opts.DataDir, constant.SqliteDBPath)
	liveConfig := filepath.Join(opts.DataDir, constant.Hysteria2ConfigPath)
	unitState, err := checkV1MigrationSource(opts, deps, binary, kernel, database)
	if err != nil {
		return err
	}
	enabled, err := migrationHY2Enabled(database, false)
	if err != nil {
		return err
	}
	if enabled {
		if err := waitMigrationHY2(ctx, liveConfig, false, deps); err != nil {
			return fmt.Errorf("source HY2 is not ready: %w", err)
		}
	}
	lock := filepath.Join(opts.InstallDir, ".migrate-v1.lock")
	if err := os.Mkdir(lock, 0700); err != nil {
		return fmt.Errorf("acquire migration lock %s (if interrupted, recover the prior snapshot before retrying): %w", lock, err)
	}
	defer os.Remove(lock)

	work, err := os.MkdirTemp(opts.InstallDir, ".migrate-v1-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	// Stage private copies so activation uses the exact verified files.
	for _, name := range []string{BinaryAsset, ChecksumAsset, pairManifestAsset, pairKernelAsset} {
		if err := copyFile(filepath.Join(assets, name), filepath.Join(work, name), 0600); err != nil {
			return fmt.Errorf("stage migration asset %s: %w", name, err)
		}
	}
	candidate, candidateKernel := filepath.Join(work, BinaryAsset), filepath.Join(work, pairKernelAsset)
	sums := filepath.Join(work, ChecksumAsset)
	if err := verifyPairExecutable(candidate); err != nil {
		return fmt.Errorf("target panel: %w", err)
	}
	if err := verifyCandidate(candidate, sums, opts.Version, deps); err != nil {
		return err
	}
	if _, err := verifyReleasePairFiles(opts.Version, candidate, sums, filepath.Join(work, pairManifestAsset), candidateKernel, deps); err != nil {
		return fmt.Errorf("target panel/kernel pair: %w", err)
	}
	snap, err := createSnapshot(opts, opts.Version, deps)
	if err != nil {
		return fmt.Errorf("prepare v1 snapshot: %w", err)
	}
	state, _ := json.Marshal(map[string]string{"activeState": "active", "unitFileState": unitState})
	if err := os.WriteFile(filepath.Join(snap.Dir, "service-state.json"), state, 0600); err != nil {
		return err
	}
	fmt.Fprintf(opts.Output, "Verified v1.0.0 and target %s. Recovery snapshot: %s\n", opts.Version, snap.Dir)
	latestState, err := checkV1MigrationSource(opts, deps, binary, kernel, database)
	if err != nil {
		return err
	}
	if latestState != unitState {
		return errors.New("source systemd boot state changed while preparing migration")
	}
	currentEnabled, err := migrationHY2Enabled(database, false)
	if err != nil || currentEnabled != enabled {
		return errors.New("source HY2 enable state changed while preparing migration")
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintln(opts.Output, "Maintenance window: stopping h-ui and its HY2 child.")
	if _, err := deps.Command("systemctl", "stop", opts.ServiceName); err != nil {
		return migrationRestartUnchanged(opts, deps, fmt.Errorf("stop old h-ui: %w", err))
	}
	if err := migrationRequireStopped(opts, deps); err != nil {
		return migrationRestartUnchanged(opts, deps, err)
	}
	// Refresh the already prepared backup after the old writer has stopped.
	// The first consistent copy stays intact until the replacement is complete.
	stoppedDB := filepath.Join(snap.Dir, "h_ui.db.stopped")
	if err := deps.BackupDatabase(database, stoppedDB); err != nil {
		return migrationRestartUnchanged(opts, deps, fmt.Errorf("snapshot stopped SQLite database: %w", err))
	}
	if err := os.Rename(stoppedDB, snap.Files[database]); err != nil {
		return migrationRestartUnchanged(opts, deps, fmt.Errorf("commit stopped SQLite snapshot: %w", err))
	}
	if err := writeUpgradeRecord(snap.Dir, migrationSource, opts.Version, "activating-pair", deps.Now()); err != nil {
		return migrationRestartUnchanged(opts, deps, err)
	}

	applyErr := func() error {
		if err := copyFile(candidateKernel, kernel, 0755); err != nil {
			return fmt.Errorf("activate target kernel: %w", err)
		}
		if err := copyFile(candidate, binary, 0755); err != nil {
			return fmt.Errorf("activate target panel: %w", err)
		}
		// Startup migrates the database and regenerates the managed HY2 YAML.
		if _, err := deps.Command("systemctl", "start", opts.ServiceName); err != nil {
			return fmt.Errorf("start target h-ui: %w", err)
		}
		if err := waitForHealthyPanel(opts, deps); err != nil {
			return fmt.Errorf("target panel health: %w", err)
		}
		isEnabled, err := migrationHY2Enabled(database, true)
		if err != nil {
			return err
		}
		if isEnabled != enabled {
			return errors.New("target HY2 enable state changed")
		}
		if enabled {
			if err := waitMigrationHY2(ctx, liveConfig, true, deps); err != nil {
				return fmt.Errorf("target HY2 health: %w", err)
			}
		}
		if !serviceActive(opts.ServiceName, deps) {
			return errors.New("target h-ui service is no longer active")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return writeUpgradeRecord(snap.Dir, migrationSource, opts.Version, "success", deps.Now())
	}()
	if applyErr == nil {
		fmt.Fprintf(opts.Output, "Migration successful: running %s with its matching HY2 kernel. Snapshot retained at %s\n", opts.Version, snap.Dir)
		return nil
	}
	// Migration has no in-place previous binary. Use rollback's snapshot fallback.
	if err := rollback(filepath.Join(work, "previous-unused"), binary, true, snap, opts, deps); err != nil {
		return fmt.Errorf("migration failed: %v; CRITICAL: rollback failed: %w; snapshot: %s", applyErr, err, snap.Dir)
	}
	if enabled {
		if err := waitMigrationHY2(context.Background(), liveConfig, false, deps); err != nil {
			return fmt.Errorf("migration failed: %v; v1 files and panel restored but old HY2 health failed: %w; snapshot: %s", applyErr, err, snap.Dir)
		}
	}
	_ = writeUpgradeRecord(snap.Dir, migrationSource, opts.Version, "rolled-back", deps.Now())
	return fmt.Errorf("migration failed: %v; rollback successful: v1.0.0 and its original kernel are running; snapshot: %s", applyErr, snap.Dir)
}

func checkV1MigrationSource(opts Options, deps Dependencies, binary, kernel, database string) (string, error) {
	for _, path := range []string{opts.InstallDir, opts.StateDir, opts.SystemdUnitPath, binary, kernel, database} {
		if resolved, err := filepath.EvalSymlinks(path); err != nil || resolved != path {
			return "", fmt.Errorf("unsupported v1 installation layout: expected a non-symlink at %s", path)
		}
	}
	if err := verifyFileDigest(binary, migrationPanelSHA); err != nil {
		return "", fmt.Errorf("only the official Better HY2 v1.0.0 panel is a supported source: %w", err)
	}
	if err := verifyFileDigest(kernel, migrationKernelSHA); err != nil {
		return "", fmt.Errorf("only the official v2.13.0-hui.4 kernel is a supported source: %w", err)
	}
	out, err := deps.Command("systemctl", "show", opts.ServiceName, "--property=ActiveState,UnitFileState,FragmentPath,DropInPaths,WorkingDirectory,Environment,EnvironmentFiles,ExecStart,ExecStartPre,ExecStartPost,ExecCondition,ExecStop,ExecStopPost,User,Type,KillMode,RootDirectory,RootImage,BindPaths,BindReadOnlyPaths,PassEnvironment,UnsetEnvironment")
	if err != nil {
		return "", fmt.Errorf("inspect source systemd unit: %w", err)
	}
	properties := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			properties[key] = value
		}
	}
	expectedEnvironment := "HUI_DATA=" + opts.DataDir + "/ GIN_MODE=release"
	expectedArgv := "argv[]=" + binary + " -p " + DefaultPanelPort + " ;"
	if properties["FragmentPath"] != opts.SystemdUnitPath || properties["WorkingDirectory"] != opts.InstallDir ||
		properties["Environment"] != expectedEnvironment || properties["User"] != "root" || properties["Type"] != "simple" ||
		properties["KillMode"] != "control-group" || strings.Count(properties["ExecStart"], "argv[]=") != 1 ||
		!strings.Contains(properties["ExecStart"], expectedArgv) {
		return "", errors.New("unsupported systemd layout: migration requires the standard v1.0.0 h-ui.service on port 8000")
	}
	for _, property := range []string{"DropInPaths", "EnvironmentFiles", "ExecStartPre", "ExecStartPost", "ExecCondition", "ExecStop", "ExecStopPost", "RootDirectory", "RootImage", "BindPaths", "BindReadOnlyPaths", "PassEnvironment", "UnsetEnvironment"} {
		if properties[property] != "" {
			return "", fmt.Errorf("unsupported h-ui.service customization: %s", property)
		}
	}
	if properties["ActiveState"] != "active" || (properties["UnitFileState"] != "enabled" && properties["UnitFileState"] != "disabled") {
		return "", errors.New("migration requires a running v1.0.0 service with enabled or disabled boot state")
	}
	return properties["UnitFileState"], nil
}

func migrationRequireStopped(opts Options, deps Dependencies) error {
	output, err := deps.Command("systemctl", "show", opts.ServiceName, "--property=ActiveState", "--value")
	if err != nil || strings.TrimSpace(string(output)) != "inactive" {
		return errors.New("systemd did not confirm that the old service stopped")
	}
	return nil
}

func migrationRestartUnchanged(opts Options, deps Dependencies, cause error) error {
	if err := restartAndVerifyPrevious(opts, deps); err != nil {
		return fmt.Errorf("migration stopped before replacement: %v; CRITICAL: could not restart v1: %w", cause, err)
	}
	return fmt.Errorf("migration stopped before replacement: %w; v1.0.0 is running", cause)
}

func migrationHY2Enabled(path string, migrated bool) (bool, error) {
	address := url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", address.String())
	if err != nil {
		return false, err
	}
	defer db.Close()
	var setting string
	if err := db.QueryRow("SELECT value FROM config WHERE key = ?", constant.Hysteria2Enable).Scan(&setting); err != nil || (setting != "0" && setting != "1") {
		return false, errors.New("cannot read HY2 enable state from SQLite")
	}
	if migrated {
		var admins int
		if err := db.QueryRow("SELECT count(*) FROM panel_admin").Scan(&admins); err != nil {
			return false, errors.New("target administrator schema was not applied")
		}
	}
	return setting == "1", nil
}

func waitMigrationHY2(ctx context.Context, path string, paired bool, deps Dependencies) error {
	// ACME DNS issuance can take several minutes. Bound the window even if the
	// kernel never gets far enough to publish its health endpoint.
	attempts := 60
	if data, err := os.ReadFile(path); err == nil {
		var config bo.Hysteria2ServerConfig
		if yaml.Unmarshal(data, &config) == nil && config.ACME != nil && config.ACME.DNS != nil {
			attempts = 320
		}
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = checkMigrationHY2(path, paired, deps)
		if lastErr == nil {
			return nil
		}
		if attempt+1 < attempts {
			deps.Sleep(time.Second)
		}
	}
	return lastErr
}

func checkMigrationHY2(path string, paired bool, deps Dependencies) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New("managed HY2 configuration is unavailable")
	}
	var config bo.Hysteria2ServerConfig
	if yaml.Unmarshal(data, &config) != nil || config.TrafficStats == nil || config.TrafficStats.Listen == nil ||
		config.TrafficStats.Secret == nil || *config.TrafficStats.Secret == "" {
		return errors.New("managed HY2 Traffic Stats configuration is unavailable")
	}
	if paired {
		if config.Auth == nil || config.Auth.HTTP == nil || config.Auth.HTTP.URL == nil {
			return errors.New("managed HY2 callback was not migrated")
		}
		endpoint, err := url.Parse(*config.Auth.HTTP.URL)
		if err != nil || endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.Port() == "" ||
			endpoint.Path != "/auth" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			return errors.New("target HY2 callback is not on the private listener")
		}
		endpoint.Path = "/ready"
		client := *deps.HTTPClient
		client.Timeout = time.Second
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		response, err := client.Get(endpoint.String())
		if err != nil {
			return errors.New("target HY2 readiness endpoint did not respond")
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			return errors.New("target HY2 has not finished startup")
		}
	}
	_, portText, err := net.SplitHostPort(*config.TrafficStats.Listen)
	if err != nil {
		return errors.New("invalid HY2 Traffic Stats listen address")
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return errors.New("invalid HY2 Traffic Stats port")
	}
	api := proxy.NewHysteria2Api(int64(port))
	secret := *config.TrafficStats.Secret
	if err := api.Ready(secret); err != nil {
		return err
	}
	if paired {
		if err := api.AccountConsistencyReady(secret); err != nil {
			return err
		}
		if !api.BandwidthReady(secret) {
			return errors.New("target kernel lacks account bandwidth capability 1")
		}
	}
	return nil
}
