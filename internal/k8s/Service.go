package k8s

import (
	"log/slog"
	"net/netip"
	"slices"

	"github.com/sncs-uk/fortigate-lb-controller/internal/config"
	corev1 "k8s.io/api/core/v1"
)

type Service struct {
	service     *corev1.Service
	client      *K8sClient
	desiredPool string
	validPool   bool

	Name        string
	Namespace   string
	Annotations map[string]string

	toAdd    []netip.Addr
	toRemove []netip.Addr
}

func (s *Service) createFromV1(service *corev1.Service, kubernetes_client *K8sClient) (err error) {
	s.client = kubernetes_client
	s.service = service
	err = s.parseV1Service()
	return
}

func (s *Service) parseV1Service() (err error) {
	err = nil
	s.Name = s.service.Name
	s.Namespace = s.service.Namespace
	s.Annotations = s.service.Annotations
	s.desiredPool, s.validPool = s.service.Annotations[config.PoolAnnotation]
	return
}

func (s *Service) IsFortigateEnabled() (enabled bool) {
	_, poolSpecified := s.service.Annotations[config.PoolAnnotation]
	_, vipV4Specified := s.service.Annotations[config.VipV4Annotation]
	_, vipV6Specified := s.service.Annotations[config.VipV6Annotation]

	return poolSpecified || vipV4Specified || vipV6Specified
}

func (s *Service) WantsIpv4() bool {
	return slices.Index(s.service.Spec.IPFamilies, "IPv4") > -1
}
func (s *Service) NeedsIpv4() bool {
	return s.WantsIpv4() && !s.HasIpv4()
}
func (s *Service) HasIpv4() bool {
	for _, ingress := range s.service.Status.LoadBalancer.Ingress {
		ip := ingress.IP
		addr, err := netip.ParseAddr(ip)
		if err == nil && addr.Is4() {
			return true
		}
	}
	return false
}

func (s *Service) WantsIpv6() bool {
	return slices.Index(s.service.Spec.IPFamilies, "IPv6") > -1
}
func (s *Service) NeedsIpv6() bool {
	return s.WantsIpv6() && !s.HasIpv6()
}
func (s *Service) HasIpv6() bool {
	for _, ingress := range s.service.Status.LoadBalancer.Ingress {
		ip := ingress.IP
		addr, err := netip.ParseAddr(ip)
		if err == nil && addr.Is6() {
			return true
		}
	}
	return false
}

func (s *Service) GetExternalAddresses() (ips []netip.Addr) {
	for _, ingress := range s.service.Status.LoadBalancer.Ingress {
		ip := ingress.IP
		addr, err := netip.ParseAddr(ip)
		if err == nil {
			ips = append(ips, addr)
		}
	}
	return
}

func (s *Service) GetExternalAddressV4() (ip netip.Addr) {
	for _, ip = range s.GetExternalAddresses() {
		if ip.Is4() {
			return
		}
	}
	return netip.Addr{}
}
func (s *Service) GetExternalAddressV6() (ip netip.Addr) {
	for _, ip = range s.GetExternalAddresses() {
		if ip.Is6() {
			return
		}
	}
	return netip.Addr{}
}
func (s *Service) GetInternalAddress() (ips []netip.Addr) {
	for _, ip := range s.service.Spec.ClusterIPs {
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			continue
		}
		ips = append(ips, addr)
	}
	return
}
func (s *Service) GetInternalAddressV4() (ip netip.Addr) {
	for _, ip = range s.GetInternalAddress() {
		if ip.Is4() {
			return
		}
	}

	return netip.Addr{}
}
func (s *Service) GetInternalAddressV6() (ip netip.Addr) {
	for _, ip = range s.GetInternalAddress() {
		if ip.Is6() {
			return
		}
	}

	return netip.Addr{}
}
func (s *Service) HasExternalAddress(address netip.Addr) bool {
	return slices.Index(s.GetExternalAddresses(), address) > -1
}
func (s *Service) AddExternalAddress(address netip.Addr) (ok bool) {
	if !address.IsValid() {
		return false
	}
	s.toAdd = append(s.toAdd, address)
	return true
}
func (s *Service) RemoveExternalAddress(address netip.Addr) bool {
	if !s.HasExternalAddress(address) {
		return false
	}
	s.toRemove = append(s.toRemove, address)
	return true
}
func (s *Service) RemoveAllExternalAddresses() {
	for _, addr := range s.GetExternalAddresses() {
		s.RemoveExternalAddress(addr)
	}
}

func (s *Service) GetVipNameV4() (name string, ok bool) {
	name, ok = s.Annotations[config.VipV4Annotation]
	return
}
func (s *Service) GetVipNameV6() (name string, ok bool) {
	name, ok = s.Annotations[config.VipV6Annotation]
	return
}

func (s *Service) GetPool() (pool string, ok bool) {
	return s.desiredPool, s.validPool
}

func (s *Service) commitStatus() (ok bool) {
	ok = true
	if len(s.toAdd)+len(s.toRemove) == 0 {
		slog.Debug("Refusing to do a null update on service", slog.String("service", s.Name))
		return
	}
	for _, address := range s.toAdd {
		if s.HasExternalAddress(address) {
			continue
		}
		slog.Debug("Adding address", slog.String("service", s.Name), slog.String("address", address.String()))
		s.service.Status.LoadBalancer.Ingress = append(s.service.Status.LoadBalancer.Ingress, corev1.LoadBalancerIngress{IP: address.String()})
	}
	for _, address := range s.toRemove {
		if !s.HasExternalAddress(address) {
			continue
		}
		slog.Debug("Removing address", slog.String("service", s.Name), slog.String("address", address.String()))
		s.service.Status.LoadBalancer.Ingress = slices.Delete(s.service.Status.LoadBalancer.Ingress, slices.Index(s.GetExternalAddresses(), address), slices.Index(s.GetExternalAddresses(), address)+1)
	}
	err := s.client.updateServiceStatus(s.service)
	if err != nil {
		slog.Warn("Error updating service status", slog.String("service", s.Name), slog.String("error", err.Error()))
		ok = false
	}
	return
}

func (s *Service) commitSpec() (ok bool) {
	ok = true
	difference := false
	for key, value := range s.service.Annotations {
		val, ok := s.Annotations[key]
		if !ok {
			difference = true
		}
		if val != value {
			difference = true
		}
	}
	if !difference {
		return
	}
	s.service.Annotations = s.Annotations
	s.client.updateService(s.service)
	return
}

func (s *Service) Commit() (ok bool) {
	statusOk := s.commitStatus()

	specOk := s.commitSpec()

	s.client.fetchService(s.service)
	s.parseV1Service()

	ok = statusOk && specOk
	return
}

func (s *Service) CheckExistingLbIps(pools map[string]*IpPool) {
	for _, ip := range s.GetExternalAddresses() {
		if (ip.Is4() && !s.WantsIpv4()) || (ip.Is6() && !s.WantsIpv6()) {
			slog.Debug("Removing IP from service, as not in ipFamilies", slog.String("service", s.Name), slog.String("ip", ip.String()))
			s.RemoveExternalAddress(ip)
			continue
		}
		pool := pools[s.desiredPool]
		if pool.Contains(ip) {
			slog.Debug("Pool contains IP", slog.String("pool", pool.Name), slog.String("address", ip.String()))
			continue
		}
		slog.Debug("Removing IP from service, as not in pool", slog.String("service", s.Name), slog.String("pool", s.desiredPool), slog.String("ip", ip.String()))
		s.RemoveExternalAddress(ip)
	}

	s.Commit()
}

func (s *Service) AssignServiceIps(pools *IpPoolList) (ipv4 netip.Addr, ipv6 netip.Addr, ok bool) {
	pool, _ := pools.GetByName(s.desiredPool)

	ok = true

	if s.NeedsIpv6() {
		addressv6, err := pool.Assign("::")
		if err != nil {
			slog.Warn("Error assigning address", slog.String("service", s.Name), slog.String("pool", pool.Name), slog.String("error", err.Error()))
			ok = false
		} else {
			slog.Debug("Assigning IPv6 address", slog.String("service", s.Name), slog.String("pool", pool.Name), slog.String("address", addressv6))
			ipv6 = netip.MustParseAddr(addressv6)
		}
	}
	if s.NeedsIpv4() {
		addressv4, err := pool.Assign("0.0.0.0")
		if err != nil {
			slog.Warn("Error assigning address", slog.String("service", s.Name), slog.String("pool", pool.Name), slog.String("error", err.Error()))
			ok = false
		} else {
			slog.Debug("Assigning IPv4 address", slog.String("service", s.Name), slog.String("pool", pool.Name), slog.String("address", addressv4))
			ipv4 = netip.MustParseAddr(addressv4)
		}
	}
	return
}
