package server

import (
	"fmt"

	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/golang/glog"

	"github.com/sczachariah/weblogic-operator/pkg/constants"
	"github.com/sczachariah/weblogic-operator/pkg/resources/services"
	"github.com/sczachariah/weblogic-operator/pkg/resources/statefulsets"
	"github.com/sczachariah/weblogic-operator/pkg/types"
)

// HasServerNameLabel returns true if the given labels map matches the given
// server name.
func HasServerNameLabel(labels map[string]string, servername string) bool {
	for label, value := range labels {
		if label == constants.WeblogicServerLabel {
			if value == servername {
				return true
			}
		}
	}
	return false
}

// Return a label that uniquely identifies a Weblogic server
func getLabelSelectorForServer(server *types.WeblogicServer) string {
	return fmt.Sprintf("%s=%s", constants.WeblogicServerLabel, server.Name)
}

// GetStatefulSetForWeblogicServer finds the associated StatefulSet for a Weblogic server
func GetStatefulSetForWeblogicServer(server *types.WeblogicServer, kubeClient kubernetes.Interface) (*v1beta1.StatefulSet, error) {
	opts := metav1.ListOptions{LabelSelector: getLabelSelectorForServer(server)}
	statefulsets, err := kubeClient.AppsV1beta1().StatefulSets(server.Namespace).List(opts)
	if err != nil {
		glog.Errorf("Unable to list stateful sets for %s: %s", server.Name, err)
		return nil, err
	}

	for _, ss := range statefulsets.Items {
		if HasServerNameLabel(ss.Labels, server.Name) {
			return &ss, nil
		}
	}
	return nil, nil
}

// CreateStatefulSetForWeblogicServer will create a new Kubernetes StatefulSet based on a predefined template
func CreateStatefulSetForWeblogicServer(clientset kubernetes.Interface, server *types.WeblogicServer, service *v1.Service) (*v1beta1.StatefulSet, error) {
	// Find StatefulSet and if it does not exist create it
	existingStatefulSet, err := GetStatefulSetForWeblogicServer(server, clientset)
	if err != nil {
		glog.Errorf("Error finding stateful set for server: %v", err)
		return nil, err
	}

	if existingStatefulSet != nil {
		glog.V(2).Infof("Stateful set with label %s already exists", getLabelSelectorForServer(server))
		return existingStatefulSet, nil
	}

	glog.V(4).Infof("Creating a new stateful set for server %s", server.Name)
	ss := statefulsets.NewForServer(server, service.Name)

	glog.V(4).Infof("Creating server %+v", ss)
	return clientset.AppsV1beta1().StatefulSets(server.Namespace).Create(ss)
}

// DeleteStatefulSetForWeblogicServer will delete a stateful set by name
func DeleteStatefulSetForWeblogicServer(clientset kubernetes.Interface, server *types.WeblogicServer) error {
	statefulSet, err := GetStatefulSetForWeblogicServer(server, clientset)
	if err != nil || statefulSet == nil {
		glog.Errorf("Could not delete stateful set: %s", err)
		return err
	}

	glog.V(4).Infof("Deleting stateful set %s", statefulSet.Name)
	var policy = metav1.DeletePropagationBackground
	return clientset.AppsV1beta1().
		StatefulSets(server.Namespace).
		Delete(statefulSet.Name, &metav1.DeleteOptions{PropagationPolicy: &policy})
}

func createWeblogicServer(server *types.WeblogicServer, kubeClient kubernetes.Interface, restClient *rest.RESTClient) error {
	server.EnsureDefaults()

	err := server.Validate()
	if err != nil {
		return err
	}

	// Validate that a label is set on the server
	if !HasServerNameLabel(server.Labels, server.Name) {
		glog.V(4).Infof("Setting label on server %s", getLabelSelectorForServer(server))
		if server.Labels == nil {
			server.Labels = make(map[string]string)
		}
		server.Labels[constants.WeblogicServerLabel] = server.Name
		return updateWeblogicServer(server, restClient)
	}

	serverService, err := CreateServiceForWeblogicServer(kubeClient, server)
	if err != nil {
		return err
	}

	_, err = CreateStatefulSetForWeblogicServer(kubeClient, server, serverService)
	if err != nil {
		return err
	}

	return nil
}

func updateWeblogicServer(server *types.WeblogicServer, restClient *rest.RESTClient) error {
	// TODO(apryde): Use retry.RetryOnConflict()?
	result := restClient.Put().
		Resource(types.ServerCRDResourcePlural).
		Namespace(server.Namespace).
		Name(server.Name).
		Body(server).
		Do()
	return result.Error()
}

// When delete server is called we will delete the stateful set (which also deletes the associated service)
//TODO handling to call stopWeblogic.sh needs to be done here
func deleteWeblogicServer(server *types.WeblogicServer, kubeClient kubernetes.Interface, restClient *rest.RESTClient) error {
	err := server.Validate()
	if err != nil {
		return err
	}

	err = DeleteStatefulSetForWeblogicServer(kubeClient, server)
	if err != nil {
		return err
	}

	err = DeleteServiceForWeblogicServer(kubeClient, server)
	if err != nil {
		return err
	}

	return nil
}

// GetServiceForWeblogicServer returns the associated service for a given server
func GetServiceForWeblogicServer(server *types.WeblogicServer, clientset kubernetes.Interface) (*v1.Service, error) {
	opts := metav1.ListOptions{LabelSelector: getLabelSelectorForServer(server)}
	services, err := clientset.CoreV1().Services(server.Namespace).List(opts)
	if err != nil {
		glog.Errorf("Unable to list services for %s: %s", server.Name, err)
		return nil, err
	}

	for _, svc := range services.Items {
		if HasServerNameLabel(svc.Labels, server.Name) {
			return &svc, nil
		}
	}
	return nil, nil
}

// CreateServiceForWeblogicServer will create a new Kubernetes Service based on a predefined template
func CreateServiceForWeblogicServer(clientset kubernetes.Interface, server *types.WeblogicServer) (*v1.Service, error) {
	// Find Service and if it does not exist create it
	existingService, err := GetServiceForWeblogicServer(server, clientset)
	if err != nil {
		glog.Errorf("Error finding service for server: %s", err)
		return nil, err
	}

	if existingService != nil {
		glog.V(2).Infof("Service with label %s already exists", getLabelSelectorForServer(server))
		return existingService, nil
	}

	glog.V(4).Infof("Creating a new service for server %s", server.Name)

	svc := services.NewForServer(server)
	return clientset.CoreV1().Services(server.Namespace).Create(svc)
}

// DeleteServiceForWeblogicServer deletes the Service associated with a Weblogic server.
func DeleteServiceForWeblogicServer(clientset kubernetes.Interface, server *types.WeblogicServer) error {
	service, err := GetServiceForWeblogicServer(server, clientset)
	if err != nil || service == nil {
		glog.Errorf("Could not delete service: %s", err)
		return err
	}
	glog.V(4).Infof("Deleting service %s", service.Name)
	return clientset.CoreV1().Services(server.Namespace).Delete(service.Name, nil)
}

func GetServerForStatefulSet(statefulSet *v1beta1.StatefulSet, restClient *rest.RESTClient) (server *types.WeblogicServer, err error) {
	if weblogicServerName, ok := statefulSet.Labels[constants.WeblogicServerLabel]; ok {
		server = &types.WeblogicServer{}
		result := restClient.Get().
			Resource(types.ServerCRDResourcePlural).
			Namespace(statefulSet.Namespace).
			Name(weblogicServerName).
			Do().
			Into(server)
		return server, result
	}
	return nil, fmt.Errorf("unable to get Label %s from statefulset. Not part of server", constants.WeblogicServerLabel)
}

func setWeblogicServerState(server *types.WeblogicServer, restClient *rest.RESTClient, phase types.WeblogicServerPhase, err error) error {
	modified := false
	if server.Status.Phase != phase {
		server.Status.Phase = phase
		modified = true
	}

	l := len(server.Status.Errors)
	if err != nil && (l < 1 || server.Status.Errors[l-1] != err.Error()) {
		server.Status.Errors = append(server.Status.Errors, err.Error())
		modified = true
	} else if l == 0 {
		server.Status.Errors = []string{}
		modified = true
	}

	// TODO(apryde): Use retry.RetryOnConflict()?
	if modified {
		result := restClient.Put().
			Resource(types.ServerCRDResourcePlural).
			Namespace(server.Namespace).
			Name(server.Name).
			Body(server).
			Do()

		return result.Error()
	}

	return nil
}

func updateServerWithStatefulSet(server *types.WeblogicServer, statefulSet *v1beta1.StatefulSet, kubeClient kubernetes.Interface, restClient *rest.RESTClient) (err error) {
	// Some simple logic for the time being.
	// To add
	// connection to the server
	// validate each pod?
	// Check how a rolling upgrade effects this
	// check version of each pod

	if statefulSet.Status.ReadyReplicas < statefulSet.Status.Replicas {
		setWeblogicServerState(server, restClient, types.WeblogicServerPending, nil)
	} else if statefulSet.Status.ReadyReplicas == statefulSet.Status.Replicas {
		setWeblogicServerState(server, restClient, types.WeblogicServerRunning, nil)
	}
	return err
}
