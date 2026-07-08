# Fortigate LB Controller

This repo houses a Kubernetes Load Balancer controller for integration with a 
FortiGate firewall. It monitors services that are deployed and adds VIPs to 
a FortiGate firewall to allow external access to the services.

## Installation
Installation is done via Helm. The chart is published as an OCI image under
`ghcr.io/sncs-uk/helm-fortigate-lb-controller`

To install, run 
`helm upgrade --install fortigate-lb-controller oci://ghcr.io/sncs-uk/helm-fortigate-lb-controller --version 0.1.1`