package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

var _ = runtime.Object(&WebLogicDomain{})
var DomainRESTClient *rest.RESTClient

const (
	defaultDomainVersion            = "12.2.1.2"
	defaultDomainReplicas           = 1
	defaultDomainManagedServerCount = 1
)

type Server struct {
	Host       string `json:"host"`
	ServerName string `json:"serverName"`
	PodName    string `json:"podName"`
	Port       int32  `json:"port"`
}

// WebLogicManagedServerSpec defines the attributes a user can specify when creating a server
type WebLogicDomainSpec struct {
	// Version defines the Weblogic Docker image version
	Version            string   `json:"version"`
	ManagedServerCount int      `json:"managedServerCount"`
	ServersAvailable   []Server `json:"serversAvailable"`
	ServersRunning     []Server `json:"serversRunning"`
	// Replicas defines the number of running Weblogic server instances
	Replicas int32 `json:"replicas,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// WebLogicDomain represents a doamin spec and associated metadata
type WebLogicDomain struct {
	metav1.TypeMeta         `json:",inline"`
	metav1.ObjectMeta       `json:"metadata"`
	Spec WebLogicDomainSpec `json:"spec"`
}

type WebLogicDomainList struct {
	metav1.TypeMeta        `json:",inline"`
	metav1.ListMeta        `json:"metadata"`
	Items []WebLogicDomain `json:"items"`
}

// EnsureDefaults will ensure that if a user omits and fields in the
// spec that are required, we set some sensible defaults.
// For example a user can choose to omit the version
// and number of replicas
func (c *WebLogicDomain) EnsureDefaults() *WebLogicDomain {
	if c.Spec.ManagedServerCount == 0 {
		c.Spec.ManagedServerCount = defaultDomainManagedServerCount
	}

	if c.Spec.Replicas == 0 || c.Spec.Replicas > 1 {
		c.Spec.Replicas = defaultDomainReplicas
	}

	if c.Spec.Version == "" {
		c.Spec.Version = defaultDomainVersion
	}

	return c
}

func (c *WebLogicDomain) GetObjectKind() schema.ObjectKind {
	return &c.TypeMeta
}

func (c *WebLogicDomainList) GetObjectKind() schema.ObjectKind {
	return &c.TypeMeta
}

func NewDomainRESTClient(config *rest.Config) (*rest.RESTClient, error) {
	//if err := types.AddToScheme(scheme.Scheme); err != nil {
	//	return nil, err
	//}
	config.GroupVersion = &WebLogicDomainSchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme.Scheme)}

	DomainRESTClient, _ = rest.RESTClientFor(config)
	return rest.RESTClientFor(config)
}
