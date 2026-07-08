package fortigate

import (
	"fmt"
	"log/slog"
	"net/netip"
	"regexp"

	forticlient "github.com/sncs-uk/fortigate-sdk-go/sdk/sdkcore"
)

type Vip struct {
	vip       *forticlient.JSONFirewallObjectVip
	vip6      *forticlient.JSONFirewallObjectVip6
	Owner     string
	Service   string
	Namespace string
	Extip     string
	Intip     string
	family    string
}

func CreateVip(externalip netip.Addr, internalip netip.Addr, owner string, service string, namespace string) (vip *Vip) {
	vip = new(Vip)
	if externalip.Is6() {
		vip.family = "6"
		vip.vip6 = new(forticlient.JSONFirewallObjectVip6)
	} else {
		vip.family = "4"
		vip.vip = new(forticlient.JSONFirewallObjectVip)
	}
	vip.Owner = owner
	vip.Service = service
	vip.Namespace = namespace
	vip.Extip = externalip.String()
	vip.Intip = internalip.String()
	return
}

func (v *Vip) Name() (name string) {
	if v.family == "4" {
		return v.vip.Name
	}
	return v.vip6.Name
}

func (v *Vip) fromJsonObject4(jsonVip *forticlient.JSONFirewallObjectVip) (err error) {
	err = nil
	v.family = "4"
	v.vip = jsonVip
	v.Extip = jsonVip.Extip
	v.Intip = jsonVip.Mappedip[0].Range
	v.parseDescription()
	return
}
func (v *Vip) fromJsonObject6(jsonVip *forticlient.JSONFirewallObjectVip6) (err error) {
	err = nil
	v.family = "6"
	v.vip6 = jsonVip
	v.Extip = jsonVip.Extip
	v.Intip = jsonVip.Mappedip
	v.parseDescription()
	return
}

func (v *Vip) parseDescription() {
	var description string
	if v.family == "6" {
		description = v.vip6.Comment
	} else {
		description = v.vip.Comment
	}
	re := regexp.MustCompile(`^([A-Za-z0-9\-]+)/([A-Za-z0-9\-]+)/([A-Za-z0-9\-]+)$`)

	parts := re.FindAllStringSubmatch(description, -1)

	if parts == nil {
		slog.Debug("vip has malformed comment", slog.String("vip", v.vip.Name))
		return
	}
	v.Owner = parts[0][1]
	v.Namespace = parts[0][2]
	v.Service = parts[0][3]
}

func (v *Vip) Save(client *FortigateClient) (err error) {
	_, err = netip.ParseAddr(v.Extip)
	if err != nil {
		return
	}
	_, err = netip.ParseAddr(v.Intip)
	if err != nil {
		return
	}
	if v.family == "4" {
		v.vip.Name = fmt.Sprintf("v-%s", v.Extip)
		v.vip.Mappedip = forticlient.VIPMultValues{}
		v.vip.Mappedip = append(v.vip.Mappedip, forticlient.VIPMultValue{Range: v.Intip})
		v.vip.Extip = v.Extip
		v.vip.Extintf = "any"
		v.vip.Comment = fmt.Sprintf("%s/%s/%s", v.Owner, v.Namespace, v.Service)
		return client.updateOrCreateVip(v.vip)
	} else {
		v.vip6.Name = fmt.Sprintf("v-%s", v.Extip)
		v.vip6.Mappedip = v.Intip
		v.vip6.Extip = v.Extip
		v.vip6.Extintf = "any"
		v.vip6.Comment = fmt.Sprintf("%s/%s/%s", v.Owner, v.Namespace, v.Service)
		return client.updateOrCreateVip6(v.vip6)
	}
}
