package upgrade

import (
	"bufio"
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"h-ui/model/constant"
)

const cliLinkPath = "/usr/local/bin/hui"

type uninstallCommand func(...string) (string, error)

type uninstallPlan struct {
	service bool
	link    bool
	files   []string
}

func Uninstall(ctx context.Context, purge bool, in io.Reader, out io.Writer) error {
	if runtime.GOOS != "linux" || currentEUID() != 0 {
		return errors.New("uninstall requires root on Linux with systemd")
	}
	pid1, err := os.ReadFile("/proc/1/comm")
	if err != nil || strings.TrimSpace(string(pid1)) != "systemd" {
		return errors.New("uninstall requires systemd as PID 1")
	}
	return runUninstall(DefaultOptions(), purge, in, out, func(args ...string) (string, error) {
		output, err := exec.CommandContext(ctx, "systemctl", args...).CombinedOutput()
		return string(output), err
	})
}

func runUninstall(opts Options, purge bool, in io.Reader, out io.Writer, systemctl uninstallCommand) error {
	plan, err := inspectUninstall(opts, purge, systemctl)
	if err != nil {
		return err
	}
	binary := filepath.Join(opts.InstallDir, "h-ui")
	fmt.Fprintln(out, "HY2 is stopped with the panel when it is a child of h-ui.service. HY2 kernel/configuration and independent Xray/HY2 services and files are retained because exclusive ownership is not recorded.")
	fmt.Fprintln(out, "Any unrelated hui command, custom data paths and external certificates are retained.")
	if plan.service {
		fmt.Fprintf(out, "Stop and disable %s; remove %s\n", opts.ServiceName, opts.SystemdUnitPath)
	}
	for _, file := range plan.files {
		fmt.Fprintf(out, "Remove %s\n", file)
	}
	if plan.link {
		fmt.Fprintf(out, "Remove %s -> %s\n", cliLinkPath, binary)
	}
	if !purge {
		fmt.Fprintln(out, "Retain databases, configuration, logs and backups. For a later purge, keep a copy of this executable and run that copy with uninstall --purge.")
	}
	if !plan.service && !plan.link && len(plan.files) == 0 {
		fmt.Fprintln(out, "No Better HY2 uninstall items remain.")
		return nil
	}
	word := "UNINSTALL"
	if purge {
		word = "PURGE"
	}
	fmt.Fprintf(out, "Type %s to confirm: ", word)
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() || scanner.Text() != word {
		fmt.Fprintln(out, "Cancelled. No changes made.")
		return scanner.Err()
	}

	current, err := inspectUninstall(opts, purge, systemctl)
	if err != nil {
		return fmt.Errorf("uninstall plan changed; no changes made: %w", err)
	}
	if current.service != plan.service || current.link != plan.link || !slices.Equal(current.files, plan.files) {
		return errors.New("uninstall plan changed; run uninstall again to review it")
	}

	if err := applyUninstall(opts, current, systemctl); err != nil {
		fmt.Fprintln(out, "Uninstall incomplete. Remaining items:")
		for _, file := range append(current.files, opts.SystemdUnitPath) {
			if _, statErr := os.Lstat(file); !os.IsNotExist(statErr) {
				fmt.Fprintln(out, file)
			}
		}
		if current.link {
			if _, statErr := os.Lstat(cliLinkPath); !os.IsNotExist(statErr) {
				fmt.Fprintln(out, cliLinkPath)
			}
		}
		if current.service {
			state, stateErr := systemctl("show", "--no-pager", "-p", "LoadState", "-p", "ActiveState", "-p", "UnitFileState", opts.ServiceName)
			if stateErr != nil {
				fmt.Fprintf(out, "%s: status unavailable\n", opts.ServiceName)
			} else {
				fmt.Fprintf(out, "%s: %s\n", opts.ServiceName, strings.TrimSpace(state))
			}
		}
		return err
	}
	fmt.Fprintln(out, "Better HY2 uninstall complete.")
	return nil
}

func inspectUninstall(opts Options, purge bool, systemctl uninstallCommand) (uninstallPlan, error) {
	var plan uninstallPlan
	binary := filepath.Join(opts.InstallDir, "h-ui")
	for _, path := range []string{opts.SystemdUnitPath, binary, cliLinkPath} {
		if err := checkUninstallPath(path, false); err != nil {
			return plan, err
		}
	}
	unit, err := os.Lstat(opts.SystemdUnitPath)
	switch {
	case err == nil:
		if !unit.Mode().IsRegular() {
			return plan, fmt.Errorf("preserving non-regular service unit: %s", opts.SystemdUnitPath)
		}
		content, err := os.ReadFile(opts.SystemdUnitPath)
		if err != nil {
			return plan, err
		}
		if !standardUninstallUnit(opts, string(content)) {
			return plan, fmt.Errorf("preserving custom or unrelated service unit: %s", opts.SystemdUnitPath)
		}
		plan.service = true
	case !os.IsNotExist(err):
		return plan, err
	}

	state, err := uninstallServiceState(opts.ServiceName, systemctl)
	if err != nil {
		return plan, err
	}
	if state["DropInPaths"] != "" ||
		(state["FragmentPath"] != "" && state["FragmentPath"] != opts.SystemdUnitPath) {
		return plan, fmt.Errorf("preserving overridden or independently installed service: %s", opts.ServiceName)
	}
	if plan.service && (state["LoadState"] != "loaded" || state["NeedDaemonReload"] != "no") {
		return plan, fmt.Errorf("cannot verify the currently loaded service matches %s", opts.SystemdUnitPath)
	}
	if !plan.service {
		if state["ActiveState"] != "inactive" && state["ActiveState"] != "failed" {
			return plan, fmt.Errorf("preserving service without a verified unit: %s", opts.ServiceName)
		}
		// An interrupted removal may leave systemd's cached unit loaded.
		plan.service = state["FragmentPath"] == opts.SystemdUnitPath
		if !plan.service && state["LoadState"] != "not-found" {
			return plan, fmt.Errorf("preserving unrecognized service: %s", opts.ServiceName)
		}
	}

	link, err := os.Readlink(cliLinkPath)
	if err == nil {
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(cliLinkPath), link)
		}
		plan.link = filepath.Clean(link) == binary
	}
	if info, err := os.Lstat(binary); err == nil {
		if !info.Mode().IsRegular() {
			return plan, fmt.Errorf("preserving non-regular program: %s", binary)
		}
		build, err := buildinfo.ReadFile(binary)
		if err != nil || build.Path != "h-ui" || build.Main.Path != "h-ui" {
			return plan, fmt.Errorf("preserving unrecognized program: %s", binary)
		}
		plan.files = append(plan.files, binary)
	} else if !os.IsNotExist(err) {
		return plan, err
	}

	if purge {
		// Keep the kernel, certificates outside these paths, and Xray's
		// service, kernel, configuration and CA. Their exclusive use is unknown.
		paths := []string{
			filepath.Join(opts.DataDir, constant.SqliteDBDir),
			filepath.Join(opts.DataDir, constant.LogDir),
			filepath.Join(opts.DataDir, constant.ExportPathDir),
			filepath.Join(opts.StateDir, "backups"),
		}
		for _, entry := range paths {
			p := filepath.Clean(entry)
			if err := checkUninstallPath(p, true); err != nil {
				return plan, err
			}
			info, err := os.Lstat(p)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return plan, err
			}
			if !info.IsDir() {
				return plan, fmt.Errorf("preserving unexpected file type: %s", p)
			}
			plan.files = append(plan.files, p)
		}
	}
	return plan, nil
}

func standardUninstallUnit(opts Options, content string) bool {
	content = strings.ReplaceAll(strings.TrimSpace(content), "\r\n", "\n")
	content = strings.Replace(content, "\nUser=root\n", "\n", 1)
	content = strings.Replace(content, "WorkingDirectory="+opts.InstallDir+"/\n", "WorkingDirectory="+opts.InstallDir+"\n", 1)
	expected := fmt.Sprintf(`[Unit]
Description=Better HY2
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=%s
Environment=HUI_DATA=%s/
Environment=GIN_MODE=release
ExecStart=%s/h-ui -p %s
Restart=on-failure
RestartSec=3s
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target`, opts.InstallDir, opts.DataDir, opts.InstallDir, DefaultPanelPort)
	return content == expected
}

func uninstallServiceState(unit string, systemctl uninstallCommand) (map[string]string, error) {
	output, err := systemctl("show", "--no-pager", "-p", "LoadState", "-p", "FragmentPath", "-p", "DropInPaths", "-p", "ActiveState", "-p", "NeedDaemonReload", unit)
	state := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			state[key] = value
		}
	}
	if err != nil && state["LoadState"] != "not-found" {
		return nil, fmt.Errorf("inspect %s: %w", unit, err)
	}
	return state, nil
}

func checkUninstallPath(path string, checkMounts bool) error {
	parent := filepath.Dir(path)
	resolved, err := filepath.EvalSymlinks(parent)
	for os.IsNotExist(err) && parent != "/" {
		parent = filepath.Dir(parent)
		resolved, err = filepath.EvalSymlinks(parent)
	}
	if err != nil || resolved != parent {
		return fmt.Errorf("preserving path with inaccessible or redirected parent: %s", path)
	}
	if !checkMounts {
		return nil
	}
	mounts, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(mounts), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		mount := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, "\\").Replace(fields[4])
		if mount == path || strings.HasPrefix(mount, path+"/") {
			return fmt.Errorf("preserving mounted data: %s", mount)
		}
	}
	return nil
}

func applyUninstall(opts Options, plan uninstallPlan, systemctl uninstallCommand) error {
	control := func(args ...string) error {
		output, err := systemctl(args...)
		if err != nil {
			return fmt.Errorf("systemctl %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(output))
		}
		return nil
	}
	if plan.service {
		if err := control("stop", opts.ServiceName); err != nil {
			return err
		}
		if _, err := os.Lstat(opts.SystemdUnitPath); err == nil {
			if err := control("disable", opts.ServiceName); err != nil {
				return err
			}
			if err := os.Remove(opts.SystemdUnitPath); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := control("daemon-reload"); err != nil {
			return err
		}
		state, err := uninstallServiceState(opts.ServiceName, systemctl)
		if err != nil || state["LoadState"] != "not-found" || state["ActiveState"] != "inactive" {
			return fmt.Errorf("could not verify removal of %s", opts.ServiceName)
		}
	}
	binary := filepath.Join(opts.InstallDir, "h-ui")
	remove := func(file string) error {
		if err := checkUninstallPath(file, true); err != nil {
			return err
		}
		return os.RemoveAll(file)
	}
	for _, file := range plan.files {
		if file == binary {
			continue
		}
		if err := remove(file); err != nil {
			return err
		}
	}
	if plan.link {
		if err := os.Remove(cliLinkPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	// Remove the executable last so a partial operation can be retried.
	if slices.Contains(plan.files, binary) {
		return remove(binary)
	}
	return nil
}
