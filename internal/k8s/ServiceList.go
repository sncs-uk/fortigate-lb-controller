package k8s

import "log/slog"

type ServiceList struct {
	services map[string]*Service

	client *K8sClient
}

func CreateServiceList(kubernetes_client *K8sClient) (list *ServiceList) {
	list = new(ServiceList)
	list.client = kubernetes_client
	return
}

func (l *ServiceList) Fetch() (ok bool) {
	services, err := l.client.getServices()
	ok = true
	if err != nil {
		slog.Error("Could not retrieve services", slog.String("error", err.Error()))
		ok = false
		return
	}

	l.services = make(map[string]*Service)
	for _, v1service := range services.Items {
		service := new(Service)
		err = service.createFromV1(&v1service, l.client)
		if err != nil {
			slog.Debug("Unable to parse service", slog.String("service", v1service.Name))
			continue
		}
		slog.Debug("Discovered service", slog.String("service", v1service.Name))
		l.services[service.Name] = service
	}
	return
}

func (l *ServiceList) GetByName(name string) (service *Service, ok bool) {
	service, ok = l.services[name]
	return
}

func (l *ServiceList) GetByVipName(vipName string) (service *Service, ok bool) {
	var serviceVipName string
	for _, service = range l.services {
		serviceVipName, ok = service.GetVipNameV4()
		if serviceVipName == vipName && ok {
			return
		}
		serviceVipName, ok = service.GetVipNameV6()
		if serviceVipName == vipName && ok {
			return
		}
	}
	service = nil
	ok = false
	return
}

func (l *ServiceList) Items() (services []*Service) {
	for _, service := range l.services {
		services = append(services, service)
	}
	return
}
