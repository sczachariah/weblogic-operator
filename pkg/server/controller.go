package server

import (
	"time"

	"k8s.io/api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"

	"weblogic-operator/pkg/types"

	"weblogic-operator/pkg/constants"
)

type StoreToWeblogicServerLister struct {
	cache.Store
}

type StoreToWeblogicStatefulSetLister struct {
	cache.Store
}

// The WeblogicController watches the Kubernetes API for changes to Weblogic resources
type WeblogicController struct {
	client                        kubernetes.Interface
	restClient                    *rest.RESTClient
	startTime                     time.Time
	shutdown                      bool
	weblogicServerController      cache.Controller
	weblogicServerStore           StoreToWeblogicServerLister
	weblogicStatefulSetController cache.Controller
	weblogicStatefulSetStore      StoreToWeblogicStatefulSetLister
}

// NewController creates a new WeblogicController.
func NewController(kubeClient kubernetes.Interface, restClient *rest.RESTClient, resyncPeriod time.Duration, namespace string) (*WeblogicController, error) {
	m := WeblogicController{
		client:     kubeClient,
		restClient: restClient,
		startTime:  time.Now(),
	}

	weblogicServerHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    m.onAdd,
		DeleteFunc: m.onDelete,
		UpdateFunc: m.onUpdate,
	}

	watcher := cache.NewListWatchFromClient(restClient, types.ServerCRDResourcePlural, namespace, fields.Everything())
	m.weblogicServerStore.Store, m.weblogicServerController = cache.NewInformer(
		watcher,
		&types.WeblogicServer{},
		resyncPeriod,
		weblogicServerHandlers)

	statefulSetHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    m.onStatefulSetAdd,
		DeleteFunc: m.onStatefulSetDelete,
		UpdateFunc: m.onStatefulSetUpdate,
	}

	m.weblogicStatefulSetStore.Store, m.weblogicStatefulSetController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = constants.WeblogicServerLabel
				return kubeClient.AppsV1beta1().StatefulSets(namespace).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = constants.WeblogicServerLabel
				return kubeClient.AppsV1beta1().StatefulSets(namespace).Watch(options)
			},
		},
		&v1beta1.StatefulSet{},
		resyncPeriod,
		statefulSetHandler)

	return &m, nil
}

func (m *WeblogicController) onAdd(obj interface{}) {
	glog.V(4).Info("WeblogicController.onAdd() called")

	weblogicServer := obj.(*types.WeblogicServer)
	err := createWeblogicServer(weblogicServer, m.client, m.restClient)
	if err != nil {
		glog.Errorf("Failed to create weblogicServer: %s", err)
		err = setWeblogicServerState(weblogicServer, m.restClient, types.WeblogicServerFailed, err)
	}
}

func (m *WeblogicController) onDelete(obj interface{}) {
	glog.V(4).Info("WeblogicController.onDelete() called")

	weblogicServer := obj.(*types.WeblogicServer)
	err := deleteWeblogicServer(weblogicServer, m.client, m.restClient)
	if err != nil {
		glog.Errorf("Failed to delete weblogicServer: %s", err)
		err = setWeblogicServerState(weblogicServer, m.restClient, types.WeblogicServerFailed, err)
	}
}

func (m *WeblogicController) onUpdate(old, cur interface{}) {
	glog.V(4).Info("WeblogicController.onUpdate() called")
	curServer := cur.(*types.WeblogicServer)
	oldServer := old.(*types.WeblogicServer)
	if curServer.ResourceVersion == oldServer.ResourceVersion {
		// Periodic resync will send update events for all known servers.
		// Two different versions of the same server will always have
		// different RVs.
		return
	}

	err := createWeblogicServer(curServer, m.client, m.restClient)
	if err != nil {
		glog.Errorf("Failed to update server: %s", err)
		err = setWeblogicServerState(curServer, m.restClient, types.WeblogicServerFailed, err)
	}
}

func (m *WeblogicController) onStatefulSetAdd(obj interface{}) {
	glog.V(4).Info("WeblogicController.onStatefulSetAdd() called")

	statefulSet := obj.(*v1beta1.StatefulSet)

	weblogicServer, err := GetServerForStatefulSet(statefulSet, m.restClient)
	if err != nil {
		// FIXME: Should we delete the stateful set here???
		// it has no server but it has the label.
		glog.Errorf("Failed to find server for stateful set: %s(%s):%#v", statefulSet.Name, err.Error(), statefulSet.Labels)
		return
	}
	err = updateServerWithStatefulSet(weblogicServer, statefulSet, m.client, m.restClient)
	if err != nil {
		glog.Errorf("Failed to create update Server: %s", err)
	}
}

func (m *WeblogicController) onStatefulSetDelete(obj interface{}) {
	glog.V(4).Info("WeblogicController.onStatefulSetDelete() called")
	m.onStatefulSetAdd(obj)
}

func (m *WeblogicController) onStatefulSetUpdate(old, new interface{}) {
	glog.V(4).Info("WeblogicController.onStatefulSetUpdate() called")
	m.onStatefulSetAdd(new)
}

// Run the Weblogic controller
func (m *WeblogicController) Run(stopChan <-chan struct{}) {
	glog.Infof("Starting Weblogic controller")
	go m.weblogicServerController.Run(stopChan)
	go m.weblogicStatefulSetController.Run(stopChan)
	<-stopChan
	glog.Infof("Shutting down Weblogic controller")
}
