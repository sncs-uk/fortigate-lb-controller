package mainlogic

import (
	"log/slog"
	"net/netip"
	"os"
	"time"

	"github.com/sncs-uk/fortigate-lb-controller/internal/config"
	"github.com/sncs-uk/fortigate-lb-controller/internal/eslog"
	"github.com/sncs-uk/fortigate-lb-controller/internal/fortigate"
	"github.com/sncs-uk/fortigate-lb-controller/internal/k8s"
)

var kubernetes_client *k8s.K8sClient

var fortigate_client *fortigate.FortigateClient

var pools *k8s.IpPoolList
var services *k8s.ServiceList
var vips *fortigate.VipList

func Run() {
	config.LoadConfig()
	startupChecks()

	go poolStats()

	eslog.Debug("Entering loop")

	for {
		RunLoop()
		time.Sleep(2 * time.Second)
	}
}

func startupChecks() {
	var err error
	// Check fortigate connectivity
	fortigate_client, err = fortigate.Init()
	if err != nil {
		eslog.Crit("Could not initiate FortiGate client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Check Kubernetes connectivity
	kubernetes_client, err = k8s.Init()
	if err != nil {
		eslog.Crit("Could not initiate Kubernetes API client", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func GetObjects() (pools *k8s.IpPoolList, vips *fortigate.VipList, services *k8s.ServiceList) {
	eslog.Debug("Getting pools")
	pools = k8s.CreateIpPoolList(kubernetes_client)
	ok := pools.Fetch()
	if !ok {
		eslog.Crit("Could not get IP Pool list")
		os.Exit(1)
	}

	// Get VIPs
	eslog.Debug("Getting VIPs")
	vips = fortigate.CreateVipList(fortigate_client)
	ok = vips.Fetch()
	if !ok {
		eslog.Crit("Could not get VIPs")
		os.Exit(1)
	}

	// Get services
	eslog.Debug("Getting Services")
	services = k8s.CreateServiceList(kubernetes_client)
	ok = services.Fetch()
	if !ok {
		eslog.Crit("Could not get services")
		os.Exit(1)
	}
	return
}

func RunLoop() {
	// Get LB pools
	pools, vips, services = GetObjects()

	eslog.Debug("Getting VIPs")
	for _, vip := range vips.Items() {
		extip, err := netip.ParseAddr(vip.Extip)
		if err != nil {
			eslog.Warn("Could not parse VIP")
			continue
		}
		for _, pool := range pools.Items() {
			if pool.Contains(extip) {
				if !pool.MustAssign(extip) {
					// There was an error assigning this VIP, that means there's a fault somewhere
					eslog.Error("Could not mark address as used", slog.String("address", extip.String()), slog.String("pool", pool.Name))
					continue
				}
			}
		}
	}

	eslog.Debug("Processing services")
	for _, service := range services.Items() {
		poolName, ok := service.GetPool()
		if !ok {
			eslog.Debug("Invalid pool detected", slog.String("service", service.Name), slog.String("pool", poolName))
			continue
		}
		pool, ok := pools.GetByName(poolName)
		// Not a valid pool
		if !ok {
			// First we need to remove the fortigate VIPs
			vipName, ok := service.GetVipNameV4()
			if ok {
				vip, ok := vips.FindByName(vipName)
				if ok {
					// found the Vip, lets remove it
					err := fortigate_client.DeleteVip(vip)
					if err != nil {
						eslog.Warn("Unable to delete VIP", slog.String("vip", vip.Name()), slog.String("error", err.Error()))
						continue
					}
					delete(service.Annotations, config.VipV4Annotation)
				}
			}
			vipName, ok = service.GetVipNameV6()
			if ok {
				vip, ok := vips.FindByName(vipName)
				if ok {
					// found the Vip, lets remove it
					err := fortigate_client.DeleteVip(vip)
					if err != nil {
						slog.Warn("Unable to delete VIP", slog.String("vip", vip.Name()), slog.String("error", err.Error()))
						continue
					}
					delete(service.Annotations, config.VipV6Annotation)
				}
			}

			service.RemoveAllExternalAddresses()
			service.Commit()
			continue
		}
		// Valid pool, are the addresses in the right pool?
		for _, address := range service.GetExternalAddresses() {
			if !pool.Contains(address) {
				service.RemoveExternalAddress(address)
			}
		}
		service.Commit()

		if service.WantsIpv4() {
			// Ok, now check if the VIPs still exist
			v4Name, ok := service.GetVipNameV4()
			if !ok {
				// This doesn't have a VIP annotation - we should remove the v4 address
				service.RemoveExternalAddress(service.GetExternalAddressV4())
			}
			_, ok = vips.FindByName(v4Name)
			if !ok {
				eslog.Info("vip missing for service, re-creating", slog.String("service", service.Name))
				// This VIP doesn't exist, but should - create it
				vip := fortigate.CreateVip(service.GetExternalAddressV4(), service.GetInternalAddressV4(), config.Heritage, service.Name, service.Namespace)
				vip.Save(fortigate_client)
				service.Annotations[config.VipV4Annotation] = vip.Name()
			}
			_, err := pool.Assign(service.GetExternalAddressV4().String())
			if err != nil {
				eslog.Debug("Unable to assign address", slog.String("address", service.GetExternalAddressV6().String()))
			}
		}

		if service.WantsIpv6() {
			v6Name, ok := service.GetVipNameV6()
			if !ok {
				// This doesn't have a VIP annotation - we should remove the v6 address
				service.RemoveExternalAddress(service.GetExternalAddressV6())
			}
			_, ok = vips.FindByName(v6Name)
			if !ok {
				// This VIP doesn't exist, but should - create it
				eslog.Info("vip missing for service, re-creating", slog.String("service", service.Name))
				vip := fortigate.CreateVip(service.GetExternalAddressV6(), service.GetInternalAddressV6(), config.Heritage, service.Name, service.Namespace)
				vip.Save(fortigate_client)
				service.Annotations[config.VipV6Annotation] = vip.Name()
			}
			_, err := pool.Assign(service.GetExternalAddressV6().String())
			if err != nil {
				eslog.Debug("Unable to assign address", slog.String("address", service.GetExternalAddressV6().String()))
			}
		}

		service.Commit()

		ipv4, ipv6, ok := service.AssignServiceIps(pools)
		if ok {
			if ipv4.IsValid() {
				// Create the IPv4 VIP
				vip := fortigate.CreateVip(ipv4, service.GetInternalAddressV4(), config.Heritage, service.Name, service.Namespace)
				err := vip.Save(fortigate_client)
				if err == nil {
					service.AddExternalAddress(ipv4)
					service.Annotations[config.VipV4Annotation] = vip.Name()
				}
			}
			if ipv6.IsValid() {
				// Create the IPv4 VIP
				vip := fortigate.CreateVip(ipv6, service.GetInternalAddressV6(), config.Heritage, service.Name, service.Namespace)
				err := vip.Save(fortigate_client)
				if err == nil {
					service.AddExternalAddress(ipv6)
					service.Annotations[config.VipV6Annotation] = vip.Name()
				}
			}
			service.Commit()
		}
	}
	for _, vip := range vips.FindByOwner(config.Heritage) {
		_, ok := services.GetByVipName(vip.Name())
		if !ok {
			eslog.Info("Found orphaned VIP, cleaning up", slog.String("vip", vip.Name()), slog.String("service", vip.Service), slog.String("namespace", vip.Namespace))
			// This VIP is not assocaited with a service anymore, it should be removed
			fortigate_client.DeleteVip(vip)
		}
	}
}
