package k8s

import (
	"log/slog"
	"net/netip"

	"github.com/sncs-uk/fortigate-lb-controller/internal/eslog"
)

type IpPoolList struct {
	pools map[string]*IpPool

	client *K8sClient
}

func CreateIpPoolList(kubernetes_client *K8sClient) (list *IpPoolList) {
	list = new(IpPoolList)
	list.client = kubernetes_client
	return
}

func (l *IpPoolList) Fetch() (ok bool) {
	lbpools, err := l.client.getLoadBalancerPools()
	ok = true
	if err != nil {
		eslog.Error("Could not retreive LoadBalancerPools", slog.String("error", err.Error()))
		ok = false
		return
	}
	eslog.Debug("Found pools", slog.Int("pool_count", len(lbpools.Items)))

	l.pools = make(map[string]*IpPool)
	for _, lbpool := range lbpools.Items {
		eslog.Noisy("Processing pool", slog.String("pool", lbpool.Metadata.Name))
		pool := new(IpPool)
		eslog.Noisy("Parsing pool", slog.String("pool", lbpool.Metadata.Name))
		err = pool.fromCRD(&lbpool)
		if err != nil {
			eslog.Warn("Failed to parse pool", slog.Any("lb_pool", lbpool.Metadata.Name))
			continue
		}
		eslog.Debug("Discovered pool", slog.String("pool", pool.Name))
		l.pools[pool.Name] = pool
	}
	return
}

func (l *IpPoolList) GetByName(name string) (pool *IpPool, ok bool) {
	pool, ok = l.pools[name]
	return
}

func (l *IpPoolList) PoolExists(name string) (valid bool) {
	_, valid = l.GetByName(name)
	return
}

func (l *IpPoolList) Items() (pools []*IpPool) {
	for _, pool := range l.pools {
		pools = append(pools, pool)
	}
	return
}

func (l *IpPoolList) MarkAddressAsUsed(address netip.Addr) (ok bool) {
	for _, pool := range l.pools {
		if pool.Contains(address) {
			pool.Assign(address.String())
			return true
		}
	}
	return false
}
