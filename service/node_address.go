package service

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	nodeAddressEnvironment = "HUI_NODE_ADDRESS"
	nodeAddressCacheTTL    = 5 * time.Minute
)

var (
	listNetworkInterfaces = net.Interfaces
	listInterfaceAddrs    = func(networkInterface net.Interface) ([]net.Addr, error) {
		return networkInterface.Addrs()
	}
	nodeAddressNow = time.Now

	nodeAddressCache struct {
		sync.Mutex
		address   string
		expiresAt time.Time
	}
	nodeIPv6Cache struct {
		sync.Mutex
		address   string
		expiresAt time.Time
	}
)

// NodeAddress returns the public IPv4 address used by generated Hysteria2
// client configurations. An explicit environment override takes precedence
// over interface discovery and is checked on every call.
func NodeAddress() (string, error) {
	if override := strings.TrimSpace(os.Getenv(nodeAddressEnvironment)); override != "" {
		address, err := publicIPv4String(override)
		if err != nil {
			return "", fmt.Errorf("%s must be a public IPv4 address: %w", nodeAddressEnvironment, err)
		}
		return address, nil
	}

	now := nodeAddressNow()
	nodeAddressCache.Lock()
	defer nodeAddressCache.Unlock()
	if nodeAddressCache.address != "" && now.Before(nodeAddressCache.expiresAt) {
		return nodeAddressCache.address, nil
	}

	address, err := detectPublicIPv4()
	if err != nil {
		return "", err
	}
	nodeAddressCache.address = address
	nodeAddressCache.expiresAt = now.Add(nodeAddressCacheTTL)
	return address, nil
}

// NodeAddressIPv6 returns a globally routable IPv6 address from an active,
// non-loopback interface. It intentionally has no implicit IPv4 fallback.
func NodeAddressIPv6() (string, error) {
	now := nodeAddressNow()
	nodeIPv6Cache.Lock()
	defer nodeIPv6Cache.Unlock()
	if nodeIPv6Cache.address != "" && now.Before(nodeIPv6Cache.expiresAt) {
		return nodeIPv6Cache.address, nil
	}
	address, err := detectPublicIPv6()
	if err != nil {
		return "", err
	}
	nodeIPv6Cache.address = address
	nodeIPv6Cache.expiresAt = now.Add(nodeAddressCacheTTL)
	return address, nil
}

func detectPublicIPv6() (string, error) {
	interfaces, err := listNetworkInterfaces()
	if err != nil {
		return "", fmt.Errorf("list network interfaces: %w", err)
	}
	addresses := make([]net.IP, 0)
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		interfaceAddresses, err := listInterfaceAddrs(networkInterface)
		if err != nil {
			return "", fmt.Errorf("list addresses for network interface %q: %w", networkInterface.Name, err)
		}
		for _, address := range interfaceAddresses {
			ip := addressIP(address)
			if isPublicIPv6(ip) {
				addresses = append(addresses, append(net.IP(nil), ip.To16()...))
			}
		}
	}
	if len(addresses) == 0 {
		return "", errors.New("no globally routable IPv6 address found on an active non-loopback network interface")
	}
	sort.Slice(addresses, func(i, j int) bool { return bytes.Compare(addresses[i], addresses[j]) < 0 })
	return addresses[0].String(), nil
}

func isPublicIPv6(ip net.IP) bool {
	if ip == nil || ip.To4() != nil || ip.To16() == nil || !ip.IsGlobalUnicast() ||
		ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"100::/64", "2001:2::/48", "2001:10::/28", "2001:db8::/32"} {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

func detectPublicIPv4() (string, error) {
	interfaces, err := listNetworkInterfaces()
	if err != nil {
		return "", fmt.Errorf("list network interfaces: %w", err)
	}

	addresses := make([]net.IP, 0)
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		interfaceAddresses, err := listInterfaceAddrs(networkInterface)
		if err != nil {
			return "", fmt.Errorf("list addresses for network interface %q: %w", networkInterface.Name, err)
		}
		for _, address := range interfaceAddresses {
			ip := addressIP(address)
			if isPublicIPv4(ip) {
				addresses = append(addresses, append(net.IP(nil), ip.To4()...))
			}
		}
	}

	if len(addresses) == 0 {
		return "", fmt.Errorf("no public IPv4 address found on an active non-loopback network interface; set %s to override detection", nodeAddressEnvironment)
	}

	// Numeric sorting makes selection stable regardless of OS interface order.
	sort.Slice(addresses, func(i, j int) bool {
		return bytes.Compare(addresses[i], addresses[j]) < 0
	})
	return addresses[0].String(), nil
}

func addressIP(address net.Addr) net.IP {
	switch value := address.(type) {
	case *net.IPNet:
		return value.IP
	case *net.IPAddr:
		return value.IP
	default:
		ip, _, err := net.ParseCIDR(address.String())
		if err == nil {
			return ip
		}
		return net.ParseIP(address.String())
	}
}

func publicIPv4String(value string) (string, error) {
	// Reject IPv4-mapped IPv6 literals even though net.IP.To4 accepts them.
	if strings.Contains(value, ":") {
		return "", errors.New("IPv6 is not supported")
	}
	ip := net.ParseIP(value)
	if !isPublicIPv4(ip) {
		return "", errors.New("address is not a publicly routable IPv4 address")
	}
	return ip.To4().String(), nil
}

func isPublicIPv4(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return false
	}

	// IsGlobalUnicast includes several special-purpose ranges that must not be
	// selected as an Internet-reachable node address.
	return !inIPv4Range(ip, 100, 64, 0, 0, 10) && // shared address space
		!inIPv4Range(ip, 192, 0, 0, 0, 24) && // IETF protocol assignments
		!inIPv4Range(ip, 192, 0, 2, 0, 24) && // documentation
		!inIPv4Range(ip, 198, 18, 0, 0, 15) && // benchmarking
		!inIPv4Range(ip, 198, 51, 100, 0, 24) && // documentation
		!inIPv4Range(ip, 203, 0, 113, 0, 24) && // documentation
		!inIPv4Range(ip, 240, 0, 0, 0, 4) // reserved
}

func inIPv4Range(ip net.IP, a, b, c, d byte, prefixLength uint) bool {
	base := net.IPv4(a, b, c, d).To4()
	mask := net.CIDRMask(int(prefixLength), 32)
	return ip.Mask(mask).Equal(base.Mask(mask))
}
