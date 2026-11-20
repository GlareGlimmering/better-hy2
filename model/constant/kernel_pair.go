package constant

import (
	_ "embed"
	"encoding/json"
)

//go:embed hy2-pair.json
var managedHysteria2PinJSON []byte

type hysteria2PairPin struct {
	Version string
	SHA256  string
}

var managedHysteria2Pin = func() hysteria2PairPin {
	var pin hysteria2PairPin
	if err := json.Unmarshal(managedHysteria2PinJSON, &pin); err != nil || pin.Version == "" || pin.SHA256 == "" {
		panic("invalid embedded Hysteria2 pair pin")
	}
	return pin
}()

// ManagedHysteria2Pin returns the version and digest of the approved v2 paired kernel.
func ManagedHysteria2Pin() (version, sha256 string) {
	return managedHysteria2Pin.Version, managedHysteria2Pin.SHA256
}
