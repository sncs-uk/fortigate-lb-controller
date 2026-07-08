package fortigate

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/sncs-uk/fortigate-sdk-go/sdk/auth"
	forticlient "github.com/sncs-uk/fortigate-sdk-go/sdk/sdkcore"
)

const retry_count int = 5

type FortigateClient struct {
	client *forticlient.FortiSDKClient
}

func Init() (*FortigateClient, error) {
	config := &tls.Config{}

	auth := &auth.Auth{}

	var err error
	_, err = auth.GetEnvToken()
	if err != nil {
		return nil, fmt.Errorf("error GetEnvToken")
	}
	_, err = auth.GetEnvHostname()
	if err != nil {
		return nil, fmt.Errorf("error GetEnvHostname")
	}
	_, err = auth.GetEnvVdom()
	if err != nil {
		return nil, fmt.Errorf("error GetEnvVdom")
	}
	_, err = auth.GetEnvCABundle()
	if err != nil {
		return nil, fmt.Errorf("error GetEnvCABundle")
	}
	b, err := auth.GetEnvInsecure()
	if err != nil {
		return nil, fmt.Errorf("error GetEnvInsecure")
	}
	config.InsecureSkipVerify = b
	_, err = auth.GetEnvPeerAuth()
	if err != nil {
		return nil, fmt.Errorf("error GetEnvHTTPProxy")
	}
	_, err = auth.GetEnvCaCert()
	if err != nil {
		return nil, fmt.Errorf("error GetEnvHTTPProxy")
	}
	_, err = auth.GetEnvClientCert()
	if err != nil {
		return nil, fmt.Errorf("error GetEnvHTTPProxy")
	}
	_, err = auth.GetEnvHTTPProxy()
	if err != nil {
		return nil, fmt.Errorf("error GetEnvHTTPProxy")
	}

	tr := &http.Transport{
		TLSClientConfig: config,
	}
	if auth.HTTPProxy != "" {
		var httpProxy *url.URL
		httpProxy, err := url.Parse(auth.HTTPProxy)
		if err != nil {
			return nil, fmt.Errorf("error parsing HTTP proxy: %w", err)
		}
		tr.Proxy = http.ProxyURL(httpProxy)
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   time.Second * 250,
	}

	slog.Info("Connecting to FortiGate API", slog.String("host", auth.Hostname))

	forticlient, err := forticlient.NewClient(auth, client)
	if err != nil {
		slog.Error("Error connecting to FortiGate API", slog.String("error_message", err.Error()))
		return nil, err
	}

	err = forticlient.CheckUP()
	if err != nil {
		slog.Error("Error connecting to FortiGate API", slog.String("error_message", err.Error()))
		return nil, err
	}

	slog.Info("Connected to FortiGate API")

	fgclient := new(FortigateClient)
	fgclient.client = forticlient

	return fgclient, nil
}

func (c *FortigateClient) getVips() (vips []*forticlient.JSONFirewallObjectVip, err error) {
	error_count := 0

	for error_count < retry_count {
		error_count++
		vips, err = c.client.ListFirewallObjectVip()
		if err == nil {
			return
		}
		slog.Warn("Failed to retrieve VIPs", slog.Int("try", error_count), slog.Int("max_tries", retry_count))
		time.Sleep(2 * time.Second)
	}
	err = fmt.Errorf("failed to connect to fortigate")
	return
}

func (c *FortigateClient) updateVip(vip *forticlient.JSONFirewallObjectVip) (err error) {
	_, err = c.client.UpdateFirewallObjectVip(vip, vip.Name)
	return
}

func (c *FortigateClient) createVip(vip *forticlient.JSONFirewallObjectVip) (err error) {
	_, err = c.client.CreateFirewallObjectVip(vip)
	return
}

func (c *FortigateClient) updateOrCreateVip(vip *forticlient.JSONFirewallObjectVip) (err error) {
	err = c.updateVip(vip)
	if err != nil {
		err = c.createVip(vip)
	}
	return
}

func (c *FortigateClient) getVip6s() (vips []*forticlient.JSONFirewallObjectVip6, err error) {
	error_count := 0

	for error_count < retry_count {
		error_count++
		vips, err = c.client.ListFirewallObjectVip6()
		if err == nil {
			return
		}
		slog.Warn("Failed to retrieve VIPs", slog.Int("try", error_count), slog.Int("max_tries", retry_count))
		time.Sleep(2 * time.Second)
	}
	err = fmt.Errorf("failed to connect to fortigate")
	return
}

func (c *FortigateClient) updateVip6(vip *forticlient.JSONFirewallObjectVip6) (err error) {
	_, err = c.client.UpdateFirewallObjectVip6(vip, vip.Name)
	return
}

func (c *FortigateClient) createVip6(vip *forticlient.JSONFirewallObjectVip6) (err error) {
	_, err = c.client.CreateFirewallObjectVip6(vip)
	return
}

func (c *FortigateClient) updateOrCreateVip6(vip *forticlient.JSONFirewallObjectVip6) (err error) {
	err = c.updateVip6(vip)
	if err != nil {
		err = c.createVip6(vip)
	}
	return
}

func (c *FortigateClient) DeleteVip(vip *Vip) (err error) {
	if vip.family == "4" {
		return c.deleteVip4(vip)
	}
	if vip.family == "6" {
		return c.deleteVip6(vip)
	}
	return
}

func (c *FortigateClient) deleteVip4(vip *Vip) (err error) {
	return c.client.DeleteFirewallObjectVip(vip.vip.Name)
}
func (c *FortigateClient) deleteVip6(vip *Vip) (err error) {
	return c.client.DeleteFirewallObjectVip6(vip.vip6.Name)
}
