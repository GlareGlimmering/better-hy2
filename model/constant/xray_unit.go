package constant

import "fmt"

func ManagedXraySystemdUnit(currentLink, config, dataDir string) string {
	return fmt.Sprintf(`[Unit]
Description=Better HY2 managed Xray outbound
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=%s/xray run -c %s
Restart=on-failure
RestartSec=3s
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadOnlyPaths=%s
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
CapabilityBoundingSet=
AmbientCapabilities=

[Install]
WantedBy=multi-user.target
`, XrayServiceUser, XrayServiceUser, currentLink, config, dataDir)
}
