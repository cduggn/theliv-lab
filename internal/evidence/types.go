/*
 * Copyright FMR LLC <opensource@fidelity.com>
 *
 * SPDX-License-Identifier: Apache
 */
package evidence

import "time"

// Every type in this file is what crosses the HTTP boundary. None of them embeds a
// Kubernetes object, so nothing a workload author writes reaches a caller unless a
// field below copies it on purpose.

type ErrorBody struct {
	Error string `json:"error"`
}

type Window struct {
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

type EventRecord struct {
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	Type           string    `json:"type"`
	Count          int32     `json:"count"`
	Component      string    `json:"component,omitzero"`
	FirstTimestamp time.Time `json:"firstTimestamp,omitzero"`
	LastTimestamp  time.Time `json:"lastTimestamp,omitzero"`
}

type EventsPage struct {
	Scope     Scope         `json:"scope"`
	Ref       Ref           `json:"ref"`
	Window    Window        `json:"window"`
	Events    []EventRecord `json:"events"`
	Truncated bool          `json:"truncated"`
}

type RelatedSet struct {
	Scope            Scope      `json:"scope"`
	Ref              Ref        `json:"ref"`
	Owners           []Resource `json:"owners"`
	Children         []Resource `json:"children"`
	Node             *Resource  `json:"node,omitzero"`
	Service          *Resource  `json:"service,omitzero"`
	CollectionErrors []string   `json:"collectionErrors"`
}

type OwnerRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type Condition struct {
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	Reason         string    `json:"reason,omitzero"`
	Message        string    `json:"message,omitzero"`
	LastTransition time.Time `json:"lastTransition,omitzero"`
}

// Resource is the projection of one object. Exactly one summary pointer is set, chosen by Ref.Kind.
type Resource struct {
	Ref       Ref               `json:"ref"`
	Namespace NamespaceName     `json:"namespace,omitzero"`
	Labels    map[string]string `json:"labels,omitzero"`
	Created   time.Time         `json:"created,omitzero"`
	Owners    []OwnerRef        `json:"owners,omitzero"`

	Pod       *PodSummary       `json:"pod,omitzero"`
	Workload  *WorkloadSummary  `json:"workload,omitzero"`
	Job       *JobSummary       `json:"job,omitzero"`
	CronJob   *CronJobSummary   `json:"cronJob,omitzero"`
	Node      *NodeSummary      `json:"node,omitzero"`
	Service   *ServiceSummary   `json:"service,omitzero"`
	Endpoints *EndpointsSummary `json:"endpoints,omitzero"`
	Ingress   *IngressSummary   `json:"ingress,omitzero"`
}

type ContainerState struct {
	Phase      string    `json:"phase"`
	Reason     string    `json:"reason,omitzero"`
	Message    string    `json:"message,omitzero"`
	ExitCode   int32     `json:"exitCode,omitzero"`
	Signal     int32     `json:"signal,omitzero"`
	StartedAt  time.Time `json:"startedAt,omitzero"`
	FinishedAt time.Time `json:"finishedAt,omitzero"`
}

type ProbeSummary struct {
	Kind                string `json:"kind"`
	Handler             string `json:"handler"`
	InitialDelaySeconds int32  `json:"initialDelaySeconds,omitzero"`
	PeriodSeconds       int32  `json:"periodSeconds,omitzero"`
	TimeoutSeconds      int32  `json:"timeoutSeconds,omitzero"`
	FailureThreshold    int32  `json:"failureThreshold,omitzero"`
}

// ContainerSummary carries env names and never env values.
type ContainerSummary struct {
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	Init           bool              `json:"init,omitzero"`
	Ready          bool              `json:"ready"`
	RestartCount   int32             `json:"restartCount"`
	State          ContainerState    `json:"state,omitzero"`
	LastTerminated *ContainerState   `json:"lastTerminated,omitzero"`
	Requests       map[string]string `json:"requests,omitzero"`
	Limits         map[string]string `json:"limits,omitzero"`
	Probes         []ProbeSummary    `json:"probes,omitzero"`
	EnvNames       []string          `json:"envNames,omitzero"`
}

type PodSummary struct {
	Phase            string             `json:"phase"`
	Reason           string             `json:"reason,omitzero"`
	Message          string             `json:"message,omitzero"`
	NodeName         string             `json:"nodeName,omitzero"`
	QOSClass         string             `json:"qosClass,omitzero"`
	StartTime        time.Time          `json:"startTime,omitzero"`
	NodeSelector     map[string]string  `json:"nodeSelector,omitzero"`
	ImagePullSecrets []string           `json:"imagePullSecrets,omitzero"`
	Conditions       []Condition        `json:"conditions,omitzero"`
	Containers       []ContainerSummary `json:"containers"`
}

// WorkloadSummary serves Deployment, ReplicaSet, StatefulSet, and DaemonSet.
type WorkloadSummary struct {
	Desired            int32             `json:"desired"`
	Current            int32             `json:"current"`
	Ready              int32             `json:"ready"`
	Available          int32             `json:"available"`
	Updated            int32             `json:"updated"`
	Generation         int64             `json:"generation"`
	ObservedGeneration int64             `json:"observedGeneration"`
	Strategy           string            `json:"strategy,omitzero"`
	Selector           map[string]string `json:"selector,omitzero"`
	Conditions         []Condition       `json:"conditions,omitzero"`
}

type JobSummary struct {
	Active         int32       `json:"active"`
	Succeeded      int32       `json:"succeeded"`
	Failed         int32       `json:"failed"`
	StartTime      time.Time   `json:"startTime,omitzero"`
	CompletionTime time.Time   `json:"completionTime,omitzero"`
	Conditions     []Condition `json:"conditions,omitzero"`
}

type CronJobSummary struct {
	Schedule           string    `json:"schedule"`
	Suspend            bool      `json:"suspend"`
	Active             int       `json:"active"`
	LastScheduleTime   time.Time `json:"lastScheduleTime,omitzero"`
	LastSuccessfulTime time.Time `json:"lastSuccessfulTime,omitzero"`
}

type NodeSummary struct {
	Unschedulable    bool              `json:"unschedulable"`
	KubeletVersion   string            `json:"kubeletVersion,omitzero"`
	OSImage          string            `json:"osImage,omitzero"`
	ContainerRuntime string            `json:"containerRuntime,omitzero"`
	Taints           []string          `json:"taints,omitzero"`
	Capacity         map[string]string `json:"capacity,omitzero"`
	Allocatable      map[string]string `json:"allocatable,omitzero"`
	Conditions       []Condition       `json:"conditions,omitzero"`
}

type ServiceSummary struct {
	Type      string            `json:"type"`
	ClusterIP string            `json:"clusterIP,omitzero"`
	Selector  map[string]string `json:"selector,omitzero"`
	Ports     []string          `json:"ports,omitzero"`
}

type EndpointAddress struct {
	IP         string `json:"ip"`
	TargetKind string `json:"targetKind,omitzero"`
	TargetName string `json:"targetName,omitzero"`
	NodeName   string `json:"nodeName,omitzero"`
}

type EndpointsSummary struct {
	Ready    []EndpointAddress `json:"ready"`
	NotReady []EndpointAddress `json:"notReady"`
}

type IngressRule struct {
	Host    string `json:"host,omitzero"`
	Path    string `json:"path,omitzero"`
	Backend string `json:"backend"`
}

type IngressSummary struct {
	Class        string        `json:"class,omitzero"`
	Rules        []IngressRule `json:"rules,omitzero"`
	TLSHosts     []string      `json:"tlsHosts,omitzero"`
	LoadBalancer []string      `json:"loadBalancer,omitzero"`
}
