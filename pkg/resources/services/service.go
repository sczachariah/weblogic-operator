package services

import (
	"github.com/sczachariah/weblogic-operator/pkg/constants"
	"github.com/sczachariah/weblogic-operator/pkg/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewForServer will return a new NodePort Kubernetes service for a Weblogic Server
func NewForServer(server *types.WeblogicServer) *v1.Service {
	weblogicPort := v1.ServicePort{Port: 7001}
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    map[string]string{constants.WeblogicServerLabel: server.Name},
			Name:      server.Name,
			Namespace: server.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{weblogicPort},
			Selector: map[string]string{
				constants.WeblogicServerLabel: server.Name,
			},
		},
	}
	return svc
}
