package fortigate

import (
	"log/slog"
	"net/netip"
	"slices"

	forticlient "github.com/sncs-uk/fortigate-sdk-go/sdk/sdkcore"
)

type VipList struct {
	vips map[string]*Vip

	client *FortigateClient
}

func CreateVipList(fortigate_client *FortigateClient) (list *VipList) {
	list = new(VipList)
	list.client = fortigate_client
	return
}

func (l *VipList) Fetch() (ok bool) {
	list, err := l.client.getVips()
	ok = true
	if err != nil {
		slog.Error("Could not retreive VIPs", slog.String("error", err.Error()))
		ok = false
		return
	}

	l.vips = make(map[string]*Vip)
	for _, jsonVip := range list {
		vip := new(Vip)
		err = vip.fromJsonObject4(jsonVip)
		if err != nil {
			slog.Debug("Failed to parse vip", slog.String("vip", jsonVip.Name))
			continue
		}
		slog.Debug("Discovered vip", slog.String("vip", jsonVip.Name))
		l.vips[jsonVip.Name] = vip
	}
	list6, err := l.client.getVip6s()

	if err != nil {
		ok = false
		return
	}

	for _, jsonVip := range list6 {
		vip := new(Vip)
		err = vip.fromJsonObject6(jsonVip)
		if err != nil {
			slog.Debug("Failed to parse vip", slog.String("vip", jsonVip.Name))
			continue
		}
		slog.Debug("Discovered vip", slog.String("vip", jsonVip.Name))
		l.vips[jsonVip.Name] = vip
	}
	return
}

func (l *VipList) Items() (items map[string]*Vip) {
	return l.vips
}

func (v *VipList) FindByExternalAddress(address netip.Addr) (vip *Vip, ok bool) {
	for _, thisVip := range v.vips {
		if thisVip.vip.Extip == address.String() {
			return vip, true
		}
	}
	return nil, false
}
func (v *VipList) FindByInternalAddress(address netip.Addr) (vip *Vip, ok bool) {
	for _, thisVip := range v.vips {
		if slices.Index(thisVip.vip.Mappedip, forticlient.VIPMultValue{Range: address.String()}) > -1 {
			return vip, true
		}
	}
	return nil, false
}

func (v *VipList) FindByName(name string) (vip *Vip, ok bool) {
	vip, ok = v.vips[name]
	return
}

func (v *VipList) FindByOwner(owner string) (vips []*Vip) {
	for _, thisVip := range v.vips {
		if thisVip.Owner == owner {
			vips = append(vips, thisVip)
		}
	}
	return
}

func (v *VipList) FindByService(service string) (vips []*Vip) {
	for _, thisVip := range v.vips {
		if thisVip.Service == service {
			vips = append(vips, thisVip)
		}
	}
	return
}

func (v *VipList) FindByNamespace(namespace string) (vips []*Vip) {
	for _, thisVip := range v.vips {
		if thisVip.Namespace == namespace {
			vips = append(vips, thisVip)
		}
	}
	return
}
