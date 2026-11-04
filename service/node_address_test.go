package service

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestNodeAddressDiscovery(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []net.Interface
		addresses  map[int][]net.Addr
		want       string
		wantError  string
	}{
		{
			name:       "single public IPv4",
			interfaces: []net.Interface{{Index: 1, Name: "eth0", Flags: net.FlagUp}},
			addresses:  map[int][]net.Addr{1: {ipNetwork("93.184.216.34/24")}},
			want:       "93.184.216.34",
		},
		{
			name: "multiple interfaces use lowest numeric public IPv4",
			interfaces: []net.Interface{
				{Index: 7, Name: "eth7", Flags: net.FlagUp},
				{Index: 2, Name: "eth2", Flags: net.FlagUp},
			},
			addresses: map[int][]net.Addr{
				7: {ipNetwork("9.9.9.9/24")},
				2: {ipNetwork("1.1.1.1/24"), ipNetwork("10.0.0.2/8")},
			},
			want: "1.1.1.1",
		},
		{
			name: "private loopback and disabled addresses are rejected",
			interfaces: []net.Interface{
				{Index: 1, Name: "lo", Flags: net.FlagUp | net.FlagLoopback},
				{Index: 2, Name: "eth0", Flags: net.FlagUp},
				{Index: 3, Name: "eth1", Flags: 0},
			},
			addresses: map[int][]net.Addr{
				1: {ipNetwork("127.0.0.1/8")},
				2: {ipNetwork("10.0.0.2/8"), ipNetwork("172.16.1.2/16"), ipNetwork("192.168.1.2/24")},
				3: {ipNetwork("93.184.216.34/24")},
			},
			wantError: "no public IPv4 address found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore := stubNetworkInterfaces(test.interfaces, test.addresses)
			defer restore()
			t.Setenv(nodeAddressEnvironment, "")
			resetNodeAddressCache()

			got, err := NodeAddress()
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("NodeAddress() error = %v, want containing %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("NodeAddress() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("NodeAddress() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNodeAddressEnvironmentOverride(t *testing.T) {
	restore := stubNetworkInterfaces(nil, nil)
	defer restore()
	resetNodeAddressCache()
	t.Setenv(nodeAddressEnvironment, " 9.9.9.9 ")

	got, err := NodeAddress()
	if err != nil {
		t.Fatalf("NodeAddress() error = %v", err)
	}
	if got != "9.9.9.9" {
		t.Fatalf("NodeAddress() = %q, want %q", got, "9.9.9.9")
	}
}

func TestNodeAddressIPv6Discovery(t *testing.T) {
	restore := stubNetworkInterfaces(
		[]net.Interface{{Index: 1, Name: "eth0", Flags: net.FlagUp}},
		map[int][]net.Addr{1: {ipNetwork("fd00::1/64"), ipNetwork("2001:4860:4860::8888/64")}},
	)
	defer restore()
	resetNodeIPv6Cache()

	got, err := NodeAddressIPv6()
	if err != nil {
		t.Fatalf("NodeAddressIPv6() error = %v", err)
	}
	if got != "2001:4860:4860::8888" {
		t.Fatalf("NodeAddressIPv6() = %q", got)
	}
}

func TestNodeAddressIPv6Missing(t *testing.T) {
	restore := stubNetworkInterfaces(
		[]net.Interface{{Index: 1, Name: "eth0", Flags: net.FlagUp}},
		map[int][]net.Addr{1: {ipNetwork("10.0.0.1/8"), ipNetwork("fd00::1/64")}},
	)
	defer restore()
	resetNodeIPv6Cache()
	if _, err := NodeAddressIPv6(); err == nil || !strings.Contains(err.Error(), "no globally routable IPv6") {
		t.Fatalf("NodeAddressIPv6() error = %v", err)
	}
}

func TestNodeAddressRejectsInvalidEnvironmentOverride(t *testing.T) {
	t.Setenv(nodeAddressEnvironment, "192.168.1.10")
	resetNodeAddressCache()

	_, err := NodeAddress()
	if err == nil || !strings.Contains(err.Error(), nodeAddressEnvironment) {
		t.Fatalf("NodeAddress() error = %v, want invalid override error", err)
	}
}

func TestNodeAddressCacheRefresh(t *testing.T) {
	interfaces := []net.Interface{{Index: 1, Name: "eth0", Flags: net.FlagUp}}
	currentAddresses := map[int][]net.Addr{1: {ipNetwork("9.9.9.9/24")}}
	restore := stubNetworkInterfaces(interfaces, currentAddresses)
	defer restore()
	t.Setenv(nodeAddressEnvironment, "")
	resetNodeAddressCache()

	originalNow := nodeAddressNow
	defer func() { nodeAddressNow = originalNow }()
	currentTime := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	nodeAddressNow = func() time.Time { return currentTime }

	got, err := NodeAddress()
	if err != nil || got != "9.9.9.9" {
		t.Fatalf("initial NodeAddress() = %q, %v", got, err)
	}
	currentAddresses[1] = []net.Addr{ipNetwork("1.1.1.1/24")}
	currentTime = currentTime.Add(nodeAddressCacheTTL - time.Second)
	got, err = NodeAddress()
	if err != nil || got != "9.9.9.9" {
		t.Fatalf("cached NodeAddress() = %q, %v", got, err)
	}
	currentTime = currentTime.Add(2 * time.Second)
	got, err = NodeAddress()
	if err != nil || got != "1.1.1.1" {
		t.Fatalf("refreshed NodeAddress() = %q, %v", got, err)
	}
}

func stubNetworkInterfaces(interfaces []net.Interface, addresses map[int][]net.Addr) func() {
	originalInterfaces := listNetworkInterfaces
	originalAddresses := listInterfaceAddrs
	listNetworkInterfaces = func() ([]net.Interface, error) { return interfaces, nil }
	listInterfaceAddrs = func(networkInterface net.Interface) ([]net.Addr, error) {
		return addresses[networkInterface.Index], nil
	}
	return func() {
		listNetworkInterfaces = originalInterfaces
		listInterfaceAddrs = originalAddresses
	}
}

func resetNodeAddressCache() {
	nodeAddressCache.Lock()
	defer nodeAddressCache.Unlock()
	nodeAddressCache.address = ""
	nodeAddressCache.expiresAt = time.Time{}
}

func resetNodeIPv6Cache() {
	nodeIPv6Cache.Lock()
	defer nodeIPv6Cache.Unlock()
	nodeIPv6Cache.address = ""
	nodeIPv6Cache.expiresAt = time.Time{}
}

func ipNetwork(cidr string) *net.IPNet {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	network.IP = ip
	return network
}
