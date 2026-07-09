package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/sncs-uk/fortigate-lb-controller/internal/eslog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const retry_count int = 5

type K8sClient struct {
	client *kubernetes.Clientset
}

func Init() (client *K8sClient, err error) {
	client = nil
	config, err := rest.InClusterConfig()
	if err != nil {
		eslog.Error("Error getting cluster config", slog.String("error", err.Error()))
		return
	}
	eslog.Info("Connecting to kubernetes API")
	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		eslog.Error("Error connecting to kuberetes API", slog.String("error", err.Error()))
		return
	}

	client = new(K8sClient)

	client.client = clientset

	eslog.Info("Connected to Kubernetes API")

	return client, nil
}

func (c *K8sClient) getServices() (services *corev1.ServiceList, err error) {
	services = nil
	error_count := 0
	for error_count < retry_count {
		error_count++
		services, err = c.client.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
		if err == nil {
			return
		}
		eslog.Warn("Failed to retreive services", slog.Int("try", error_count), slog.Int("max_tries", retry_count))
		time.Sleep(2 * time.Second)
	}
	err = fmt.Errorf("failed to connect to kubernetes service")
	return
}

func (c *K8sClient) fetchService(service *corev1.Service) (err error) {
	service, err = c.client.CoreV1().Services(service.Namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	eslog.Debug("Fetched service", slog.String("service", service.Name))
	return
}

func (c *K8sClient) getLoadBalancerPools() (results *LoadBalancerPoolList, err error) {
	results = nil
	error_count := 0
	for error_count < retry_count {
		error_count++
		var crds []byte
		crds, err = c.client.RESTClient().Get().AbsPath("/apis/sncs-uk.io/v1beta1/lbpools").DoRaw(context.TODO())
		if err == nil {
			results = new(LoadBalancerPoolList)
			err = json.Unmarshal(crds, results)
			return
		}
		eslog.Warn("Failed to retreive lbpools", slog.Int("try", error_count), slog.Int("max_tries", retry_count))
		time.Sleep(2 * time.Second)
	}
	err = fmt.Errorf("failed to connect to kubernetes service")
	return
}

func (c *K8sClient) updateServiceStatus(service *corev1.Service) (err error) {
	eslog.Debug("Updating service status", slog.Any("service", service))
	_, err = c.client.CoreV1().Services(service.Namespace).UpdateStatus(context.TODO(), service, metav1.UpdateOptions{})
	return err
}

func (c *K8sClient) updateService(service *corev1.Service) (err error) {
	eslog.Debug("Updaing service", slog.Any("service", service))
	_, err = c.client.CoreV1().Services(service.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	return err
}
