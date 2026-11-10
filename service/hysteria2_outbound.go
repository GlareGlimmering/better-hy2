package service

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/entity"
	"h-ui/model/vo"
	"h-ui/util"
)

var installedHysteria2Version = util.InstalledHysteria2Version

// PrepareHysteria2OutboundConfig derives userOutbounds from the account table.
// Legacy installations remain unchanged while every active regular account is
// unbound. Once one active account is bound, all active accounts must be bound.
func PrepareHysteria2OutboundConfig(config bo.Hysteria2ServerConfig) (bo.Hysteria2ServerConfig, error) {
	accounts, err := dao.ListAccount("role = ? and deleted = 0", "user")
	if err != nil {
		return config, err
	}
	settings, err := loadXraySettings(constant.XrayDraftConfig)
	if err != nil {
		return config, fmt.Errorf("load managed outbound nodes: %w", err)
	}
	config, err = MergeManagedOutboundNodes(config, settings)
	if err != nil {
		return config, err
	}
	return PrepareHysteria2OutboundConfigWithAccounts(config, accounts)
}

func PrepareHysteria2OutboundConfigWithAccounts(config bo.Hysteria2ServerConfig, accounts []entity.Account) (bo.Hysteria2ServerConfig, error) {
	// Never trust a userOutbounds map supplied through an imported config. The
	// account table is the single source of truth for authenticated identities.
	config.UserOutbounds = nil
	if err := ValidateHysteria2OutboundConfig(config); err != nil {
		return config, err
	}

	bindings, err := BuildUserOutboundBindings(accounts, config.Outbounds)
	if err != nil {
		return config, err
	}
	config.UserOutbounds = bindings
	if err := ValidateHysteria2OutboundConfig(config); err != nil {
		return config, err
	}
	return config, nil
}

// MergeManagedOutboundNodes materializes the user-facing node inventory into
// the Hysteria2 outbounds consumed by the customized kernel. Unmatched legacy
// rules are deliberately retained as advanced rules.
func MergeManagedOutboundNodes(config bo.Hysteria2ServerConfig, settings bo.XraySettings) (bo.Hysteria2ServerConfig, error) {
	settings = normalizeXraySettings(settings)
	managed := make(map[string]bo.ServerConfigOutboundEntry)
	directName, directType, directMode := "DIRECT", "direct", "auto"
	directFastOpen := false
	managed["direct"] = bo.ServerConfigOutboundEntry{Name: &directName, Type: &directType, Direct: &bo.ServerConfigOutboundDirect{Mode: &directMode, FastOpen: &directFastOpen}}
	for _, node := range settings.Outbounds {
		if !node.Enabled {
			continue
		}
		name, outboundType := node.Name, "socks5"
		var address string
		var username, password *string
		switch node.Protocol {
		case "vless", "http":
			address = net.JoinHostPort(node.SOCKSListen, strconv.Itoa(int(node.SOCKSPort)))
		case "socks5":
			address = net.JoinHostPort(node.Address, strconv.Itoa(int(node.Port)))
			if node.Username != "" {
				u, p := node.Username, node.Password
				username, password = &u, &p
			}
		default:
			return config, fmt.Errorf("outbound node %q has unsupported protocol %q", node.Name, node.Protocol)
		}
		entry := bo.ServerConfigOutboundEntry{Name: &name, Type: &outboundType, SOCKS5: &bo.ServerConfigOutboundSOCKS5{Addr: &address, Username: username, Password: password}}
		key := strings.ToLower(name)
		if _, exists := managed[key]; exists {
			return config, fmt.Errorf("duplicate managed outbound node %q", name)
		}
		managed[key] = entry
	}

	result := make([]bo.ServerConfigOutboundEntry, 0, len(config.Outbounds)+len(managed))
	seen := map[string]bool{}
	for _, existing := range config.Outbounds {
		if existing.Name == nil {
			result = append(result, existing)
			continue
		}
		key := strings.ToLower(strings.TrimSpace(*existing.Name))
		if replacement, ok := managed[key]; ok {
			result = append(result, replacement)
			seen[key] = true
			continue
		}
		result = append(result, existing)
		seen[key] = true
	}
	for key, entry := range managed {
		if !seen[key] {
			result = append(result, entry)
		}
	}
	config.Outbounds = result
	return config, nil
}

func EnsureHysteria2UserRoutingCompatible(config bo.Hysteria2ServerConfig) error {
	if len(config.UserOutbounds) == 0 {
		return nil
	}
	version, err := installedHysteria2Version()
	if err != nil {
		return fmt.Errorf("verify Hysteria2 user-routing kernel: %w", err)
	}
	if !supportsUserOutboundRouting(version) {
		return fmt.Errorf("Hysteria2 %s does not support managed userOutbounds; required %s", version, constant.Hysteria2CustomVersion)
	}
	return nil
}

func supportsUserOutboundRouting(version string) bool {
	return version == constant.Hysteria2CustomVersion || version == constant.Hysteria2AccountConsistencyVersion
}

func Hysteria2RoutingStatus() (vo.Hysteria2RoutingStatusVo, error) {
	status := vo.Hysteria2RoutingStatusVo{UnboundUsers: []string{}, Errors: []string{}, KernelVersion: "-"}
	config, err := GetHysteria2Config()
	if err != nil {
		return status, err
	}
	status.Enabled = len(config.UserOutbounds) > 0
	accounts, err := dao.ListAccount("role = ? and deleted = 0", "user")
	if err != nil {
		return status, err
	}
	status.ActiveUsers = len(accounts)
	for _, account := range accounts {
		if strings.TrimSpace(AccountOutbound(account)) == "" {
			if account.Username != nil {
				status.UnboundUsers = append(status.UnboundUsers, *account.Username)
			}
		} else {
			status.BoundUsers++
		}
	}
	base := config
	base.UserOutbounds = nil
	if err := ValidateHysteria2OutboundConfig(base); err != nil {
		status.Errors = append(status.Errors, err.Error())
	}
	if err := validateStrictUserRoutingCompatibility(config); err != nil {
		status.Errors = append(status.Errors, err.Error())
	}
	bindings, bindingErr := BuildUserOutboundBindings(accounts, config.Outbounds)
	if bindingErr != nil {
		status.Errors = append(status.Errors, bindingErr.Error())
	} else if len(bindings) > 0 {
		known, err := outboundNames(config.Outbounds)
		if err == nil {
			for username, requested := range bindings {
				if _, ok := known[strings.ToLower(requested)]; !ok {
					status.Errors = append(status.Errors, fmt.Sprintf("user %q references unknown outbound %q", username, requested))
					break
				}
			}
		}
	}
	version, versionErr := installedHysteria2Version()
	if versionErr != nil {
		status.Errors = append(status.Errors, "kernel: "+versionErr.Error())
	} else {
		status.KernelVersion = version
		status.KernelCompatible = supportsUserOutboundRouting(version)
		if !status.KernelCompatible {
			status.Errors = append(status.Errors, fmt.Sprintf("kernel %s is incompatible; required %s", version, constant.Hysteria2CustomVersion))
		}
	}
	if status.ActiveUsers == 0 {
		status.Errors = append(status.Errors, "no active regular users")
	}
	status.Ready = status.ActiveUsers > 0 && status.BoundUsers == status.ActiveUsers && len(bindings) > 0 && status.KernelCompatible && len(status.Errors) == 0
	return status, nil
}

func BuildUserOutboundBindings(accounts []entity.Account, outbounds []bo.ServerConfigOutboundEntry) (map[string]string, error) {
	active := make([]entity.Account, 0, len(accounts))
	routingEnabled := false
	for _, account := range accounts {
		if account.Role == nil || *account.Role != "user" || (account.Deleted != nil && *account.Deleted != 0) {
			continue
		}
		active = append(active, account)
		if strings.TrimSpace(AccountOutbound(account)) != "" {
			routingEnabled = true
		}
	}
	if !routingEnabled {
		return nil, nil
	}

	known, err := outboundNames(outbounds)
	if err != nil {
		return nil, err
	}
	bindings := make(map[string]string, len(active))
	for _, account := range active {
		if account.Username == nil || *account.Username == "" {
			return nil, errors.New("active account has an empty username")
		}
		requested := strings.TrimSpace(AccountOutbound(account))
		if requested == "" {
			return nil, fmt.Errorf("account %q has no outbound binding", *account.Username)
		}
		canonical, ok := known[strings.ToLower(requested)]
		if !ok {
			return nil, fmt.Errorf("account %q references unknown outbound %q", *account.Username, requested)
		}
		bindings[*account.Username] = canonical
	}
	return bindings, nil
}

func ValidateHysteria2OutboundConfig(config bo.Hysteria2ServerConfig) error {
	known, err := outboundNames(config.Outbounds)
	if err != nil {
		return err
	}
	for _, entry := range config.Outbounds {
		if err := validateOutboundEntry(entry); err != nil {
			return err
		}
	}

	if len(config.UserOutbounds) == 0 {
		return nil
	}
	if err := validateStrictUserRoutingCompatibility(config); err != nil {
		return err
	}
	for username, requested := range config.UserOutbounds {
		if username == "" {
			return errors.New("user outbound binding has an empty username")
		}
		if _, ok := known[strings.ToLower(requested)]; !ok {
			return fmt.Errorf("user %q references unknown outbound %q", username, requested)
		}
	}
	return nil
}

func validateStrictUserRoutingCompatibility(config bo.Hysteria2ServerConfig) error {
	if config.ACL != nil && ((config.ACL.File != nil && strings.TrimSpace(*config.ACL.File) != "") || len(config.ACL.Inline) > 0) {
		return errors.New("user outbound routing cannot be combined with ACL")
	}
	if config.Resolver != nil && config.Resolver.Type != nil {
		resolverType := strings.ToLower(strings.TrimSpace(*config.Resolver.Type))
		if resolverType != "" && resolverType != "system" {
			return errors.New("user outbound routing only supports the system resolver")
		}
	}
	return nil
}

func OutboundInUse(name string, accounts []entity.Account) bool {
	for _, account := range accounts {
		if strings.EqualFold(strings.TrimSpace(AccountOutbound(account)), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func ValidateAccountOutbound(name string) error {
	_, err := CanonicalAccountOutbound(name)
	return err
}

func CanonicalAccountOutbound(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	if strings.EqualFold(name, "DIRECT") {
		return "DIRECT", nil
	}
	if settings, settingsErr := loadXraySettings(constant.XrayDraftConfig); settingsErr == nil {
		for _, node := range settings.Outbounds {
			if node.Enabled && strings.EqualFold(node.Name, name) {
				return node.Name, nil
			}
		}
	}
	config, err := GetHysteria2Config()
	if err != nil {
		return "", err
	}
	if err := ValidateHysteria2OutboundConfig(config); err != nil {
		return "", err
	}
	known, err := outboundNames(config.Outbounds)
	if err != nil {
		return "", err
	}
	canonical, ok := known[strings.ToLower(name)]
	if !ok {
		return "", fmt.Errorf("unknown outbound %q", name)
	}
	return canonical, nil
}

func outboundNames(outbounds []bo.ServerConfigOutboundEntry) (map[string]string, error) {
	known := make(map[string]string, len(outbounds))
	for _, entry := range outbounds {
		if entry.Name == nil || strings.TrimSpace(*entry.Name) == "" {
			return nil, errors.New("outbound name is required")
		}
		name := strings.TrimSpace(*entry.Name)
		if name != *entry.Name {
			return nil, fmt.Errorf("outbound name %q must not contain surrounding whitespace", *entry.Name)
		}
		if len(name) > 64 {
			return nil, fmt.Errorf("outbound name %q exceeds 64 characters", name)
		}
		key := strings.ToLower(name)
		if _, exists := known[key]; exists {
			return nil, fmt.Errorf("duplicate outbound name %q", name)
		}
		known[key] = name
	}
	return known, nil
}

func validateOutboundEntry(entry bo.ServerConfigOutboundEntry) error {
	name := ""
	if entry.Name != nil {
		name = strings.TrimSpace(*entry.Name)
	}
	if entry.Type == nil {
		return fmt.Errorf("outbound %q type is required", name)
	}
	if *entry.Type != strings.TrimSpace(*entry.Type) {
		return fmt.Errorf("outbound %q type must not contain surrounding whitespace", name)
	}
	switch strings.ToLower(*entry.Type) {
	case "direct":
		if entry.Direct == nil {
			return fmt.Errorf("outbound %q direct settings are required", name)
		}
		mode := ""
		if entry.Direct.Mode != nil {
			mode = strings.ToLower(strings.TrimSpace(*entry.Direct.Mode))
		}
		if mode != "" && mode != "auto" && mode != "64" && mode != "46" && mode != "6" && mode != "4" {
			return fmt.Errorf("outbound %q has invalid direct mode %q", name, mode)
		}
		if entry.Direct.BindDevice != nil && strings.TrimSpace(*entry.Direct.BindDevice) != "" &&
			((entry.Direct.BindIPv4 != nil && strings.TrimSpace(*entry.Direct.BindIPv4) != "") ||
				(entry.Direct.BindIPv6 != nil && strings.TrimSpace(*entry.Direct.BindIPv6) != "")) {
			return fmt.Errorf("outbound %q cannot bind both device and IP", name)
		}
		if entry.Direct.BindIPv4 != nil && strings.TrimSpace(*entry.Direct.BindIPv4) != "" {
			ip := net.ParseIP(strings.TrimSpace(*entry.Direct.BindIPv4))
			if ip == nil || ip.To4() == nil {
				return fmt.Errorf("outbound %q has invalid bindIPv4", name)
			}
		}
		if entry.Direct.BindIPv6 != nil && strings.TrimSpace(*entry.Direct.BindIPv6) != "" {
			ip := net.ParseIP(strings.TrimSpace(*entry.Direct.BindIPv6))
			if ip == nil || ip.To4() != nil {
				return fmt.Errorf("outbound %q has invalid bindIPv6", name)
			}
		}
	case "socks5":
		if entry.SOCKS5 == nil || entry.SOCKS5.Addr == nil {
			return fmt.Errorf("outbound %q SOCKS5 address is required", name)
		}
		if *entry.SOCKS5.Addr != strings.TrimSpace(*entry.SOCKS5.Addr) {
			return fmt.Errorf("outbound %q SOCKS5 address must not contain surrounding whitespace", name)
		}
		if err := validateHostPort(*entry.SOCKS5.Addr); err != nil {
			return fmt.Errorf("outbound %q has invalid SOCKS5 address: %w", name, err)
		}
	case "http":
		// Retain compatibility with previously imported official Hysteria2
		// configurations. The managed UI will expose only DIRECT and SOCKS5.
		if entry.HTTP == nil || entry.HTTP.URL == nil {
			return fmt.Errorf("outbound %q HTTP URL is required", name)
		}
		parsed, err := url.ParseRequestURI(*entry.HTTP.URL)
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("outbound %q has invalid HTTP URL", name)
		}
	default:
		return fmt.Errorf("outbound %q has unsupported type %q", name, *entry.Type)
	}
	return nil
}

func validateHostPort(address string) error {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil || strings.TrimSpace(host) == "" {
		return errors.New("expected host:port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}
