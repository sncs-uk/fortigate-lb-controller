package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/sncs-uk/fortigate-lb-controller/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const retry_count int = 5

type K8sClient struct {
	client *kubernetes.Clientset

	deploymentName     string
	namespace          string
	serviceAccountName string

	clusterRoleBindings []*rbacv1.ClusterRoleBinding
	roleBindings        []*rbacv1.RoleBinding
	clusterRoles        []*rbacv1.ClusterRole
	roles               []*rbacv1.Role
}

func Init() (client *K8sClient, err error) {
	client = nil
	config, err := rest.InClusterConfig()
	if err != nil {
		slog.Error("Error getting cluster config: %s", slog.String("error", err.Error()))
		return
	}
	slog.Info("Connecting to kubernetes API")
	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		slog.Error("Error connecting to kuberetes API: %s", slog.String("error", err.Error()))
		return
	}

	client = new(K8sClient)

	client.client = clientset

	slog.Info("Connected to Kubernetes API")

	client.getMyDeployment()
	client.getRoles()

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
		slog.Warn("Failed to retreive services", slog.Int("try", error_count), slog.Int("max_tries", retry_count))
		time.Sleep(2 * time.Second)
	}
	err = fmt.Errorf("failed to connect to kubernetes service")
	return
}

func (c *K8sClient) fetchService(service *corev1.Service) (err error) {
	service, err = c.client.CoreV1().Services(service.Namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	slog.Debug("Fetched service", slog.String("service", service.Name))
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
		slog.Warn("Failed to retreive lbpools", slog.Int("try", error_count), slog.Int("max_tries", retry_count))
		time.Sleep(2 * time.Second)
	}
	err = fmt.Errorf("failed to connect to kubernetes service")
	return
}

func (c *K8sClient) updateServiceStatus(service *corev1.Service) (err error) {
	slog.Debug("Updating service status", slog.Any("service", service))
	_, err = c.client.CoreV1().Services(service.Namespace).UpdateStatus(context.TODO(), service, metav1.UpdateOptions{})
	return err
}

func (c *K8sClient) updateService(service *corev1.Service) (err error) {
	slog.Debug("Updaing service", slog.Any("service", service))
	_, err = c.client.CoreV1().Services(service.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	return err
}

func (c *K8sClient) getMyDeployment() (err error) {
	slog.Info("Getting my deployment")
	podNamespace := os.Getenv("POD_NAMESPACE")
	podName := os.Getenv("POD_NAME")

	var pod *corev1.Pod
	var replicaSet *appsv1.ReplicaSet
	var deployment *appsv1.Deployment

	pod, err = c.client.CoreV1().Pods(podNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		slog.Debug("Error getting pod", slog.String("error", err.Error()))
		return
	}

	replicaSet, err = c.client.AppsV1().ReplicaSets(podNamespace).Get(context.TODO(), pod.OwnerReferences[0].Name, metav1.GetOptions{})
	if err != nil {
		slog.Debug("Error getting replicaset", slog.String("error", err.Error()))
		return
	}

	deployment, err = c.client.AppsV1().Deployments(podNamespace).Get(context.Background(), replicaSet.OwnerReferences[0].Name, metav1.GetOptions{})
	if err != nil {
		slog.Debug("Error getting deployment", slog.String("error", err.Error()))
		return
	}
	c.deploymentName = deployment.Name
	c.namespace = deployment.Namespace
	c.serviceAccountName = pod.Spec.ServiceAccountName
	return
}

func (c *K8sClient) getRoles() (err error) {
	var clusterRoleBindings *rbacv1.ClusterRoleBindingList
	var roleBindings *rbacv1.RoleBindingList

	clusterRoleBindings, err = c.client.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return
	}
	roleBindings, err = c.client.RbacV1().RoleBindings(c.namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return
	}

	for _, binding := range clusterRoleBindings.Items {
		for _, subject := range binding.Subjects {
			if subject.Name == c.serviceAccountName && subject.Namespace == c.namespace {
				// This is a valid CRB
				c.clusterRoleBindings = append(c.clusterRoleBindings, &binding)
			}
		}
	}

	for _, binding := range roleBindings.Items {
		for _, subject := range binding.Subjects {
			if subject.Name == c.serviceAccountName && subject.Namespace == c.namespace {
				// This is a valid RB
				c.roleBindings = append(c.roleBindings, &binding)
			}
		}
	}

	for _, binding := range c.clusterRoleBindings {
		role, err := c.client.RbacV1().ClusterRoles().Get(context.Background(), binding.RoleRef.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		c.clusterRoles = append(c.clusterRoles, role)
	}
	for _, binding := range c.roleBindings {
		role, err := c.client.RbacV1().Roles(c.namespace).Get(context.Background(), binding.RoleRef.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		c.roles = append(c.roles, role)
	}
	return
}

func (c *K8sClient) CheckDeploymentDeleted() (deleted bool, err error) {
	var deployment *appsv1.Deployment
	deleted = false
	deployment, err = c.client.AppsV1().Deployments(c.namespace).Get(context.Background(), c.deploymentName, metav1.GetOptions{})

	if deployment.DeletionTimestamp != nil {
		// This has been deleted
		deleted = true
	}
	return
}

func (c *K8sClient) CheckFinalizers() (err error) {
	// Check the deployment first
	var deployment *appsv1.Deployment
	deployment, err = c.client.AppsV1().Deployments(c.namespace).Get(context.Background(), c.deploymentName, metav1.GetOptions{})
	if slices.Index(deployment.Finalizers, config.Finalizer) == -1 {
		deployment.Finalizers = append(deployment.Finalizers, config.Finalizer)
		_, err = c.client.AppsV1().Deployments(c.deploymentName).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	// Check the clusterrolebinding next
	for _, binding := range c.clusterRoleBindings {
		if slices.Index(binding.Finalizers, config.Finalizer) == -1 {
			binding.Finalizers = append(binding.Finalizers, config.Finalizer)
			_, err = c.client.RbacV1().ClusterRoleBindings().Update(context.TODO(), binding, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}

	// And the rolebinding next
	for _, binding := range c.roleBindings {
		if slices.Index(binding.Finalizers, config.Finalizer) == -1 {
			binding.Finalizers = append(binding.Finalizers, config.Finalizer)
			_, err = c.client.RbacV1().RoleBindings(c.namespace).Update(context.TODO(), binding, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}

	// And now the clusterroles
	for _, role := range c.clusterRoles {
		if slices.Index(role.Finalizers, config.Finalizer) == -1 {
			role.Finalizers = append(role.Finalizers, config.Finalizer)
			_, err = c.client.RbacV1().ClusterRoles().Update(context.TODO(), role, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}

	// And finally the roles
	for _, role := range c.roles {
		if slices.Index(role.Finalizers, config.Finalizer) == -1 {
			role.Finalizers = append(role.Finalizers, config.Finalizer)
			_, err = c.client.RbacV1().Roles(c.namespace).Update(context.TODO(), role, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *K8sClient) RemoveFinalizers() (err error) {
	var deployment *appsv1.Deployment
	deployment, err = c.client.AppsV1().Deployments(c.namespace).Get(context.Background(), c.deploymentName, metav1.GetOptions{})
	if err != nil {
		return
	}

	deployment.Finalizers = slices.Delete(deployment.Finalizers, slices.Index(deployment.Finalizers, config.Finalizer), slices.Index(deployment.Finalizers, config.Finalizer)+1)
	_, err = c.client.AppsV1().Deployments(c.deploymentName).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	return
}
