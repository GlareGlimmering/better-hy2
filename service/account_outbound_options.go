package service

import (
	"fmt"
	"net"
	"strings"
	"time"

	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/vo"
)

// GetAccountOutboundOptions reads applied inventory, not editable drafts. It
// never edits historical HY2 rules or changes existing account bindings.
func GetAccountOutboundOptions() (vo.AccountOutboundOptions, error) {
	active, err := loadXraySettings(constant.XrayActiveConfig)
	if err != nil {
		return vo.AccountOutboundOptions{}, err
	}
	draft, err := loadXraySettings(constant.XrayDraftConfig)
	if err != nil {
		return vo.AccountOutboundOptions{}, err
	}
	config, err := GetHysteria2Config()
	if err != nil {
		return vo.AccountOutboundOptions{}, err
	}
	return accountOutboundOptions(active, draft, config, func(address string) bool {
		conn, err := net.DialTimeout("tcp", address, 150*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}), nil
}

func accountOutboundOptions(active, draft bo.XraySettings, config bo.Hysteria2ServerConfig, localReady func(string) bool) vo.AccountOutboundOptions {
	result := vo.AccountOutboundOptions{Options: []vo.AccountOutboundOption{{Name: "DIRECT", Type: "direct", Source: "builtin"}}, Unavailable: []vo.AccountOutboundOption{}}
	seen := map[string]bool{"direct": true}
	known := map[string]bool{}
	adapterEndpoints := map[string]bool{}
	for _, node := range draft.Outbounds {
		known[strings.ToLower(node.Name)] = true
	}
	for _, node := range active.Outbounds {
		if node.Enabled && (node.Protocol == "vless" || node.Protocol == "http") {
			adapterEndpoints[net.JoinHostPort(node.SOCKSListen, fmt.Sprint(node.SOCKSPort))] = true
		}
		key := strings.ToLower(node.Name)
		known[key] = true
		if seen[key] {
			continue
		}
		seen[key] = true
		item := vo.AccountOutboundOption{Name: node.Name, Type: "socks5", Source: "managed"}
		if node.Enabled {
			result.Options = append(result.Options, item)
		} else {
			item.Reason = "disabled"
			result.Unavailable = append(result.Unavailable, item)
		}
	}
	for _, rule := range config.Outbounds {
		if rule.Name == nil || rule.Type == nil {
			continue
		}
		name := *rule.Name
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		item := vo.AccountOutboundOption{Name: name, Type: strings.ToLower(*rule.Type), Source: "native"}
		if known[key] {
			item.Source = "managed"
			item.Reason = "not_applied"
			result.Unavailable = append(result.Unavailable, item)
			continue
		}
		if item.Type != "direct" && item.Type != "socks5" {
			continue
		}
		if item.Type == "socks5" && rule.SOCKS5 != nil && rule.SOCKS5.Addr != nil {
			address := *rule.SOCKS5.Addr
			if adapterEndpoints[address] {
				item.Source = "legacy_unverified"
				item.Reason = "local_mapping_unverified"
				result.Unavailable = append(result.Unavailable, item)
				continue
			}
			host, _, err := net.SplitHostPort(address)
			if err == nil && (strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()) && !localReady(address) {
				// No ownership claim is made from an address. An unavailable local
				// endpoint is not selectable, but its original config is retained.
				item.Source = "legacy_unverified"
				item.Reason = "local_proxy_unavailable"
				result.Unavailable = append(result.Unavailable, item)
				continue
			}
		}
		result.Options = append(result.Options, item)
	}
	return result
}
