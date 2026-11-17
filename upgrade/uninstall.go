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
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"h-ui/model/constant"
)

const cliLinkPath = "/usr/local/bin/hui"

type uninstallCommand func(...string) (string, error)
type uninstallExec func(string, ...string) (string, error)

type uninstallService struct {
	name string
	path string
}

type uninstallPlan struct {
	services  []uninstallService
	link      bool
	files     []string
	dirs      []string
	preserved []string
	commands  [][]string
}

func Uninstall(ctx context.Context, purge bool, in io.Reader, out io.Writer) error {
	if runtime.GOOS != "linux" || currentEUID() != 0 {
		return errors.New("uninstall requires root on Linux with systemd")
	}
	pid1, err := os.ReadFile("/proc/1/comm")
	if err != nil || strings.TrimSpace(string(pid1)) != "systemd" {
		return errors.New("uninstall requires systemd as PID 1")
	}
	run := func(name string, args ...string) (string, error) {
		output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
		return string(output), err
	}
	return runUninstall(DefaultOptions(), purge, in, out, run)
}

func runUninstall(opts Options, purge bool, in io.Reader, out io.Writer, run uninstallExec) error {
	plan, err := inspectUninstall(opts, run)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Remove the standard Better HY2 installation and its data. External services, external certificates and shared system journal entries are outside this operation.")
	for _, item := range plan.preserved {
		fmt.Fprintf(out, "Preserve %s\n", item)
	}
	for _, service := range plan.services {
		fmt.Fprintf(out, "Stop and disable %s; remove %s\n", service.name, service.path)
	}
	for _, file := range plan.files {
		fmt.Fprintf(out, "Remove %s\n", file)
	}
	for _, dir := range plan.dirs {
		fmt.Fprintf(out, "Remove %s if empty\n", dir)
	}
	if plan.link {
		fmt.Fprintf(out, "Remove %s -> %s\n", cliLinkPath, filepath.Join(opts.InstallDir, "h-ui"))
	}
	for _, command := range plan.commands {
		fmt.Fprintf(out, "Run %s\n", strings.Join(command, " "))
	}
	if len(plan.services) == 0 && !plan.link && len(plan.files) == 0 && len(plan.dirs) == 0 && len(plan.commands) == 0 {
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
	current, err := inspectUninstall(opts, run)
	if err != nil {
		return fmt.Errorf("uninstall plan changed; no changes made: %w", err)
	}
	if !reflect.DeepEqual(current, plan) {
		return errors.New("uninstall plan changed; run uninstall again to review it")
	}
	if err := applyUninstall(opts, current, run); err != nil {
		fmt.Fprintln(out, "Uninstall incomplete. Review the error and rerun to list remaining items.")
		for _, file := range current.files {
			if _, statErr := os.Lstat(file); !os.IsNotExist(statErr) {
				fmt.Fprintln(out, file)
			}
		}
		for _, service := range current.services {
			if _, statErr := os.Lstat(service.path); !os.IsNotExist(statErr) {
				fmt.Fprintln(out, service.path)
			}
		}
		if current.link {
			if _, statErr := os.Lstat(cliLinkPath); !os.IsNotExist(statErr) {
				fmt.Fprintln(out, cliLinkPath)
			}
		}
		return err
	}
	fmt.Fprintln(out, "Better HY2 uninstall complete.")
	return nil
}

func inspectUninstall(opts Options, run uninstallExec) (uninstallPlan, error) {
	var plan uninstallPlan
	binary := filepath.Join(opts.InstallDir, "h-ui")
	if err := checkUninstallPath(cliLinkPath, false); err != nil {
		return plan, err
	}
	panel := uninstallService{opts.ServiceName, opts.SystemdUnitPath}
	removePanel, err := inspectUninstallService(panel, run, func(content string) bool { return standardUninstallUnit(opts, content) })
	if err != nil {
		return plan, err
	}
	if removePanel {
		plan.services = append(plan.services, panel)
	}
	if info, err := os.Lstat(binary); err == nil {
		if !info.Mode().IsRegular() {
			return plan, fmt.Errorf("preserving non-regular program: %s", binary)
		}
		build, err := buildinfo.ReadFile(binary)
		if err != nil || build.Path != "h-ui" || build.Main.Path != "h-ui" {
			return plan, fmt.Errorf("preserving unrecognized program: %s", binary)
		}
	} else if !os.IsNotExist(err) {
		return plan, err
	}
	if target, err := os.Readlink(cliLinkPath); err == nil {
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(cliLinkPath), target)
		}
		plan.link = filepath.Clean(target) == binary
	}
	entries, err := uninstallDirEntries(opts.InstallDir)
	if err != nil {
		return plan, err
	}
	if entries != nil {
		plan.dirs = append(plan.dirs, opts.InstallDir)
		for _, entry := range entries {
			plan.files = append(plan.files, filepath.Join(opts.InstallDir, entry.Name()))
		}
	}
	xray := uninstallService{constant.XraySystemdUnitName, filepath.Join(filepath.Dir(opts.SystemdUnitPath), constant.XraySystemdUnitName)}
	xrayDir := filepath.Join(opts.StateDir, "xray")
	expectedUnit := constant.ManagedXraySystemdUnit(filepath.Join(xrayDir, "current"), filepath.Join(xrayDir, "config.json"), xrayDir)
	removeXray, xrayErr := inspectUninstallService(xray, run, func(content string) bool { return strings.TrimSpace(content) == strings.TrimSpace(expectedUnit) })
	if xrayErr != nil {
		// A differently installed Xray may depend on these paths.
		plan.preserved = append(plan.preserved, xray.name+" and "+xrayDir+" ("+xrayErr.Error()+")")
	} else if removeXray {
		plan.services = append(plan.services, xray)
	}
	stateEntries, err := uninstallDirEntries(opts.StateDir)
	if err != nil {
		return plan, err
	}
	pruneState := stateEntries != nil
	for _, entry := range stateEntries {
		name := entry.Name()
		if name == "backups" || (name == "xray" && xrayErr == nil) {
			plan.files = append(plan.files, filepath.Join(opts.StateDir, name))
		} else {
			pruneState = false
			plan.preserved = append(plan.preserved, filepath.Join(opts.StateDir, name))
		}
	}
	if pruneState {
		plan.dirs = append(plan.dirs, opts.StateDir)
	}

	accessDir := filepath.Join(filepath.Dir(opts.StateDir), filepath.Base(filepath.Dir(constant.Hysteria2AccessLogPath)))
	logEntries, err := uninstallDirEntries(accessDir)
	if err != nil {
		return plan, err
	}
	pruneAccess := logEntries != nil
	for _, entry := range logEntries {
		if !entry.IsDir() && accessLogFile.MatchString(entry.Name()) {
			plan.files = append(plan.files, filepath.Join(accessDir, entry.Name()))
		} else {
			pruneAccess = false
		}
	}
	if pruneAccess {
		plan.dirs = append(plan.dirs, accessDir)
	}

	for _, path := range plan.files {
		if err := checkUninstallPath(path, true); err != nil {
			return plan, err
		}
	}
	if xrayErr == nil {
		if err := planXrayAccountRemoval(&plan, xrayDir, run); err != nil {
			return plan, err
		}
	}
	if err := planPortHoppingRemoval(&plan, run); err != nil {
		return plan, err
	}
	return plan, nil
}

var accessLogFile = regexp.MustCompile("^hy2_access(?:-[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}(?:[.][0-9]{3})?)?[.]log(?:[.]gz)?$")

func uninstallDirEntries(path string) ([]os.DirEntry, error) {
	if err := checkUninstallPath(path, true); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("preserving unexpected directory type: %s", path)
	}
	return os.ReadDir(path)
}

func inspectUninstallService(unit uninstallService, run uninstallExec, match func(string) bool) (bool, error) {
	if err := checkUninstallPath(unit.path, true); err != nil {
		return false, err
	}
	info, err := os.Lstat(unit.path)
	exists := err == nil
	if exists {
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("preserving non-regular service unit: %s", unit.path)
		}
		content, err := os.ReadFile(unit.path)
		if err != nil {
			return false, err
		}
		if !match(string(content)) {
			return false, fmt.Errorf("preserving custom or unrelated service unit: %s", unit.path)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	state, err := readUninstallService(run, unit.name)
	if err != nil {
		return false, err
	}
	if state["DropInPaths"] != "" || (state["FragmentPath"] != "" && state["FragmentPath"] != unit.path) {
		return false, fmt.Errorf("preserving overridden or independently installed service: %s", unit.name)
	}
	if exists {
		if state["LoadState"] != "loaded" || state["FragmentPath"] != unit.path || state["NeedDaemonReload"] != "no" {
			return false, fmt.Errorf("cannot verify the currently loaded service matches %s", unit.path)
		}
		return true, nil
	}
	if state["ActiveState"] != "inactive" && state["ActiveState"] != "failed" {
		return false, fmt.Errorf("preserving service without a verified unit: %s", unit.name)
	}
	if state["FragmentPath"] == unit.path {
		return true, nil
	}
	if state["LoadState"] != "not-found" {
		return false, fmt.Errorf("preserving unrecognized service: %s", unit.name)
	}
	return false, nil
}

func readUninstallService(run uninstallExec, name string) (map[string]string, error) {
	return uninstallServiceState(name, func(args ...string) (string, error) { return run("systemctl", args...) })
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

func applyUninstall(opts Options, plan uninstallPlan, run uninstallExec) error {
	control := func(name string, args ...string) error {
		output, err := run(name, args...)
		if err != nil {
			return fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(output))
		}
		return nil
	}
	for _, unit := range plan.services {
		if err := control("systemctl", "stop", unit.name); err != nil {
			return err
		}
	}
	for _, unit := range plan.services {
		if _, err := os.Lstat(unit.path); err == nil {
			if err := control("systemctl", "disable", unit.name); err != nil {
				return err
			}
			if err := os.Remove(unit.path); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if len(plan.services) > 0 {
		if err := control("systemctl", "daemon-reload"); err != nil {
			return err
		}
		for _, unit := range plan.services {
			state, err := readUninstallService(run, unit.name)
			if err != nil || state["LoadState"] != "not-found" || state["ActiveState"] != "inactive" {
				return fmt.Errorf("could not verify removal of %s", unit.name)
			}
		}
	}
	isNetwork := func(command []string) bool {
		return command[0] == "nft" || command[0] == "iptables" || command[0] == "ip6tables"
	}
	var remainingNetwork uninstallPlan
	if slices.ContainsFunc(plan.commands, isNetwork) {
		// A clean shutdown may already have removed a port hopping rule.
		if err := planPortHoppingRemoval(&remainingNetwork, run); err != nil {
			return err
		}
		for _, command := range remainingNetwork.commands {
			if !slices.ContainsFunc(plan.commands, func(approved []string) bool { return slices.Equal(approved, command) }) {
				return errors.New("port hopping rules changed during uninstall; review the new plan and retry")
			}
		}
	}
	for _, command := range plan.commands {
		if isNetwork(command) && !slices.ContainsFunc(remainingNetwork.commands, func(remaining []string) bool { return slices.Equal(remaining, command) }) {
			continue
		}
		if command[0] == "groupdel" {
			_, err := run("getent", "group", command[1])
			if noGetentMatch(err) {
				continue
			}
			if err != nil {
				return fmt.Errorf("check remaining Xray group: %w", err)
			}
		}
		if err := control(command[0], command[1:]...); err != nil {
			return err
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
		if file != binary {
			if err := remove(file); err != nil {
				return fmt.Errorf("remove %s: %w", file, err)
			}
		}
	}
	// Remove executable last, then prune already-empty roots.
	if slices.Contains(plan.files, binary) {
		if err := remove(binary); err != nil {
			return fmt.Errorf("remove %s: %w", binary, err)
		}
	}
	for _, dir := range plan.dirs {
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove empty directory %s: %w", dir, err)
		}
	}
	if plan.link {
		if err := os.Remove(cliLinkPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
