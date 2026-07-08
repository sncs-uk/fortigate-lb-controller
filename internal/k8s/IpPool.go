package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"

	ciliumip "github.com/cilium/cilium/pkg/ip"
)

type IpPool struct {
	Name string
	ipv4 netip.Prefix
	ipv6 netip.Prefix

	availableIpv4 []string
	availableIpv6 []string
}

func (p *IpPool) fromCRD(crd *LoadBalancerPool) (err error) {
	err = nil

	if crd.Spec.IPv4 == "" && crd.Spec.IPv6 == "" {
		err = fmt.Errorf("neither IPv4 or IPv6 prefix given")
		return
	}
	p.Name = crd.Metadata.Name
	if crd.Spec.IPv4 != "" {
		var net4 netip.Prefix
		net4, err = netip.ParsePrefix(crd.Spec.IPv4)
		if err != nil {
			return
		}
		var ips []string
		ips, err = ciliumip.PrefixToIps(net4.String(), 0)
		if err != nil {
			return
		}

		p.ipv4 = net4
		p.availableIpv4 = ips
		for _, v4 := range crd.Spec.ExcludeV4 {
			slog.Debug("Removing excluded address", slog.String("pool", p.Name), slog.String("address", v4))
			addr, _ := netip.ParseAddr(v4)
			p.removeAvailable(addr)
		}
	}
	if crd.Spec.IPv6 != "" {
		var net6 netip.Prefix
		net6, err = netip.ParsePrefix(crd.Spec.IPv6)
		if err != nil {
			return
		}
		var ips []string
		ips, err = ciliumip.PrefixToIps(net6.String(), 0)
		if err != nil {
			return
		}

		p.ipv6 = net6
		p.availableIpv6 = ips
		for _, v6 := range crd.Spec.ExcludeV6 {
			slog.Debug("Removing excluded address", slog.String("pool", p.Name), slog.String("address", v6))
			addr, _ := netip.ParseAddr(v6)
			p.removeAvailable(addr)
		}
	}
	return
}

func (p *IpPool) Contains(address netip.Addr) (contains bool) {
	contains = false
	if p.ipv4.Contains(address) {
		contains = true
	}
	if p.ipv6.Contains(address) {
		contains = true
	}
	return
}

func (p *IpPool) ContainsMultiple(address []netip.Addr) (contains bool) {
	contains = true
	for _, addr := range address {
		if !p.Contains(addr) {
			contains = false
		}
	}
	return
}

func (p *IpPool) isAvailableV4(address netip.Addr) bool {
	return slices.Contains(p.availableIpv4, address.String())
}
func (p *IpPool) isAvailableV6(address netip.Addr) bool {
	return slices.Contains(p.availableIpv6, address.String())
}

func (p *IpPool) hasV4() bool {
	return p.ipv4.IsValid()
}

func (p *IpPool) hasV6() bool {
	return p.ipv6.IsValid()
}

func (p *IpPool) sizeV4() (count int) {
	fullSize, _ := ciliumip.PrefixToIps(p.ipv4.String(), 0)
	return len(fullSize)
}
func (p *IpPool) sizeV6() (count int) {
	fullSize, _ := ciliumip.PrefixToIps(p.ipv6.String(), 0)
	return len(fullSize)
}

func (p *IpPool) usedV4() (count int) {
	return p.sizeV4() - len(p.availableIpv4)
}
func (p *IpPool) usedV6() (count int) {
	return p.sizeV6() - len(p.availableIpv6)
}

func (p *IpPool) availableV4() (count int) {
	return len(p.availableIpv4)
}
func (p *IpPool) availableV6() (count int) {
	return len(p.availableIpv6)
}

func (p *IpPool) removeAvailable(address netip.Addr) error {
	if address.Is4() {
		if slices.Contains(p.availableIpv4, address.String()) {
			slog.Debug("Marking address as used", slog.String("pool", p.Name), slog.String("address", address.String()))
			p.availableIpv4 = slices.Delete(p.availableIpv4, slices.Index(p.availableIpv4, address.String()), slices.Index(p.availableIpv4, address.String())+1)
			return nil
		}
	} else if address.Is6() {
		if slices.Contains(p.availableIpv6, address.String()) {
			slog.Debug("Marking address as used", slog.String("pool", p.Name), slog.String("address", address.String()))
			p.availableIpv6 = slices.Delete(p.availableIpv6, slices.Index(p.availableIpv6, address.String()), slices.Index(p.availableIpv6, address.String())+1)
			return nil
		}
	} else {
		return fmt.Errorf("invalid IP type")
	}
	return fmt.Errorf("that address is not available in this pool")
}

func (p *IpPool) Assign(preferred string) (string, error) {
	addr, err := netip.ParseAddr(preferred)
	if err != nil {
		return "", fmt.Errorf("invalid address: %s", preferred)
	}
	if addr.Is4() {
		if !p.hasV4() {
			// We don't have a v4
			return "", fmt.Errorf("pool does not have an IPv4 scope")
		}
		if p.Contains(addr) && p.isAvailableV4(addr) {
			// We should assign this one then
			p.removeAvailable(addr)
			return preferred, nil
		}
		address := p.availableIpv4[0]
		addr, _ = netip.ParseAddr(address)
		err = p.removeAvailable(addr)
		if err != nil {
			return "", err
		}
		return address, nil
	}
	if addr.Is6() {
		if !p.hasV6() {
			// We don't have a v4
			return "", fmt.Errorf("pool does not have an IPv6 scope")
		}
		if p.Contains(addr) && p.isAvailableV6(addr) {
			// We should assign this one then
			p.removeAvailable(addr)
			return preferred, nil
		}
		address := p.availableIpv6[0]
		addr, _ = netip.ParseAddr(address)
		err = p.removeAvailable(addr)
		if err != nil {
			return "", err
		}
		return address, nil
	}
	return "", fmt.Errorf("unrecognised IP address")
}

func (p *IpPool) MustAssign(address netip.Addr) (ok bool) {
	if address.Is4() {
		if p.isAvailableV4(address) {
			p.removeAvailable(address)
			return true
		} else {
			return false
		}
	}
	if address.Is6() {
		if p.isAvailableV4(address) {
			p.removeAvailable(address)
			return true
		} else {
			return false
		}
	}
	return false
}

func (p *IpPool) PrintStats(level slog.Level) {
	slog.Log(context.Background(), level, "Pool Stats (v4)", slog.String("pool", p.Name), slog.Int("pool_size", p.sizeV4()), slog.Int("used_addresses", p.usedV4()), slog.Int("available_addresses", p.availableV4()))
	slog.Log(context.Background(), level, "Pool Stats (v6)", slog.String("pool", p.Name), slog.Int("pool_size", p.sizeV6()), slog.Int("used_addresses", p.usedV6()), slog.Int("available_addresses", p.availableV6()))
}
