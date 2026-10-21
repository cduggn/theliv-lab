/*
 * Copyright FMR LLC <opensource@fidelity.com>
 *
 * SPDX-License-Identifier: Apache
 */
package evidence

import (
	"fmt"
	"strings"

	com "github.com/fidelity/theliv/pkg/common"
	"k8s.io/apimachinery/pkg/util/validation"
)

type ClusterName string

type NamespaceName string

// Scope is the one cluster and namespace a request may read. Build it with ParseScope.
type Scope struct {
	Cluster   ClusterName   `json:"cluster"`
	Namespace NamespaceName `json:"namespace"`
}

func ParseScope(cluster, namespace string) (Scope, error) {
	if !com.ClusterNamePattern.MatchString(cluster) {
		return Scope{}, fmt.Errorf("invalid cluster name %q", cluster)
	}
	if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
		return Scope{}, fmt.Errorf("invalid namespace %q: %s", namespace, strings.Join(errs, "; "))
	}
	return Scope{Cluster: ClusterName(cluster), Namespace: NamespaceName(namespace)}, nil
}

// Kind is the closed set of kinds the evidence API serves. Secrets and ConfigMaps have no value here.
type Kind string

const (
	KindPod         Kind = "Pod"
	KindReplicaSet  Kind = "ReplicaSet"
	KindDeployment  Kind = "Deployment"
	KindStatefulSet Kind = "StatefulSet"
	KindDaemonSet   Kind = "DaemonSet"
	KindJob         Kind = "Job"
	KindCronJob     Kind = "CronJob"
	KindService     Kind = "Service"
	KindEndpoints   Kind = "Endpoints"
	KindIngress     Kind = "Ingress"
	KindNode        Kind = "Node"
)

var allKinds = []Kind{
	KindPod, KindReplicaSet, KindDeployment, KindStatefulSet, KindDaemonSet,
	KindJob, KindCronJob, KindService, KindEndpoints, KindIngress, KindNode,
}

func AllKinds() []Kind {
	return append([]Kind(nil), allKinds...)
}

func ParseKind(s string) (Kind, error) {
	for _, k := range allKinds {
		if strings.EqualFold(s, string(k)) {
			return k, nil
		}
	}
	return "", fmt.Errorf("unsupported kind %q", s)
}

// Namespaced is false only for Node, the one cluster-scoped kind detect already shows every user.
func (k Kind) Namespaced() bool {
	return k != KindNode
}

// Ref names one object inside a Scope.
type Ref struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}

func ParseRef(kind, name string) (Ref, error) {
	k, err := ParseKind(kind)
	if err != nil {
		return Ref{}, err
	}
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return Ref{}, fmt.Errorf("invalid name %q: %s", name, strings.Join(errs, "; "))
	}
	return Ref{Kind: k, Name: name}, nil
}
