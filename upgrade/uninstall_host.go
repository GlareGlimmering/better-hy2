package upgrade

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"h-ui/model/constant"
)

func commandNotFound(err error) bool { return errors.Is(err, exec.ErrNotFound) }

func noGetentMatch(err error) bool {
	var status *exec.ExitError
	return errors.As(err, &status) && status.ExitCode() == 2
}

func planXrayAccountRemoval(plan *uninstallPlan, home string, run uninstallExec) error {
	name := constant.XrayServiceUser
	passwd, err := run("getent", "passwd", name)
	if noGetentMatch(err) {
		return nil
	}
	if err != nil {
		if commandNotFound(err) {
			plan.preserved = append(plan.preserved, "Xray service account (getent unavailable)")
			return nil
		}
		return fmt.Errorf("inspect Xray service account: %w", err)
	}
	fields := strings.Split(strings.TrimSpace(passwd), ":")
	if len(fields) != 7 || fields[0] != name || fields[5] != home || fields[6] != "/usr/sbin/nologin" {
		plan.preserved = append(plan.preserved, "unrecognized "+name+" account")
		return nil
	}
	uid, err := strconv.Atoi(fields[2])
	if err != nil || uid <= 0 || uid >= 1000 {
		plan.preserved = append(plan.preserved, "non-system "+name+" account")
		return nil
	}
	group, err := run("getent", "group", name)
	if noGetentMatch(err) {
		plan.preserved = append(plan.preserved, name+" account with a custom primary group")
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Xray service group: %w", err)
	}
	groupFields := strings.Split(strings.TrimSpace(group), ":")
	if len(groupFields) != 4 || groupFields[0] != name || groupFields[2] != fields[3] {
		plan.preserved = append(plan.preserved, name+" account with a custom primary group")
		return nil
	}
	plan.commands = append(plan.commands, []string{"userdel", name})
	if groupFields[3] != "" {
		plan.preserved = append(plan.preserved, "shared "+name+" group")
		return nil
	}
	accounts, err := run("getent", "passwd")
	if err != nil {
		return fmt.Errorf("inspect Xray group users: %w", err)
	}
	for _, record := range strings.Split(accounts, "\n") {
		entry := strings.Split(record, ":")
		if len(entry) == 7 && entry[0] != name && entry[3] == fields[3] {
			plan.preserved = append(plan.preserved, "shared "+name+" group")
			return nil
		}
	}
	// userdel usually removes a private user group on its own.
	plan.commands = append(plan.commands, []string{"groupdel", name})
	return nil
}

func firewallUnavailable(err error, output string) bool {
	message := strings.ToLower(output)
	return commandNotFound(err) || strings.Contains(message, "operation not permitted") ||
		strings.Contains(message, "permission denied") || strings.Contains(message, "table does not exist")
}

func planPortHoppingRemoval(plan *uninstallPlan, run uninstallExec) error {
	tables, err := run("nft", "list", "tables")
	if err != nil && !firewallUnavailable(err, tables) {
		return fmt.Errorf("inspect nftables: %w", err)
	}
	if err != nil && !commandNotFound(err) {
		plan.preserved = append(plan.preserved, "unverified nftables state")
	}
	for _, line := range strings.Split(tables, "\n") {
		if strings.TrimSpace(line) == "table inet "+constant.Hysteria2PortHoppingTable {
			plan.commands = append(plan.commands, []string{"nft", "delete", "table", "inet", constant.Hysteria2PortHoppingTable})
			break
		}
	}
	for _, tool := range []string{"iptables", "ip6tables"} {
		// Query existing tables without loading the NAT module or creating
		// chains during the pre-confirmation inspection.
		output, err := run(tool + "-save")
		if err != nil {
			if !firewallUnavailable(err, output) {
				return fmt.Errorf("inspect %s port hopping rules: %w", tool, err)
			}
			if !commandNotFound(err) {
				plan.preserved = append(plan.preserved, "unverified "+tool+" state")
			}
			continue
		}
		inNAT := false
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "*") {
				inNAT = line == "*nat"
				continue
			}
			if line == "COMMIT" {
				inNAT = false
			}
			if !inNAT {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 3 || fields[0] != "-A" || fields[1] != "PREROUTING" {
				continue
			}
			for i := 2; i < len(fields)-1; i++ {
				if fields[i] == "--comment" && strings.Trim(fields[i+1], "\"") == constant.Hysteria2PortHoppingComment {
					fields[0] = "-D"
					fields[i+1] = constant.Hysteria2PortHoppingComment
					prefix := []string{tool, "-w", "-t", "nat"}
					plan.commands = append(plan.commands, append(prefix, fields...))
					break
				}
			}
		}
	}
	return nil
}
