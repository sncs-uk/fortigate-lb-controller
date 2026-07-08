package k8s

import (
	"log/slog"
	"net/netip"
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
		slog.Error("Could not retreive LoadBalancerPools", slog.String("error", err.Error()))
		ok = false
		return
	}

	l.pools = make(map[string]*IpPool)
	for _, lbpool := range lbpools.Items {
		pool := new(IpPool)
		err = pool.fromCRD(&lbpool)
		if err != nil {
			slog.Warn("Failed to parse pool", slog.Any("lb_pool", lbpool.Metadata.Name))
			continue
		}
		slog.Debug("Discovered pool", slog.String("pool", pool.Name))
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
			slog.Debug("Marking address as used in pool", slog.String("address", address.String()), slog.String("pool", pool.Name))
			pool.Assign(address.String())
			return true
		}
	}
	return false
}
