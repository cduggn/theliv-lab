/*
 * Copyright FMR LLC <opensource@fidelity.com>
 *
 * SPDX-License-Identifier: Apache
 */
package evidence

import (
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Project reduces a Kubernetes object to the fields the evidence API serves.
// It is an allowlist. Anything not copied here never leaves the process.
func Project(obj runtime.Object) (Resource, error) {
	switch o := obj.(type) {
	case *corev1.Pod:
		return base(KindPod, o, func(r *Resource) { r.Pod = projectPod(o) }), nil
	case *appsv1.ReplicaSet:
		return base(KindReplicaSet, o, func(r *Resource) {
			r.Workload = &WorkloadSummary{
				Desired: deref(o.Spec.Replicas, 1), Current: o.Status.Replicas, Ready: o.Status.ReadyReplicas,
				Available: o.Status.AvailableReplicas, Generation: o.Generation, ObservedGeneration: o.Status.ObservedGeneration,
				Selector: selectorOf(o.Spec.Selector),
				Conditions: conditions(o.Status.Conditions, func(c appsv1.ReplicaSetCondition) Condition {
					return Condition{string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime.Time}
				}),
			}
		}), nil
	case *appsv1.Deployment:
		return base(KindDeployment, o, func(r *Resource) {
			r.Workload = &WorkloadSummary{
				Desired: deref(o.Spec.Replicas, 1), Current: o.Status.Replicas, Ready: o.Status.ReadyReplicas,
				Available: o.Status.AvailableReplicas, Updated: o.Status.UpdatedReplicas,
				Generation: o.Generation, ObservedGeneration: o.Status.ObservedGeneration,
				Strategy: string(o.Spec.Strategy.Type), Selector: selectorOf(o.Spec.Selector),
				Conditions: conditions(o.Status.Conditions, func(c appsv1.DeploymentCondition) Condition {
					return Condition{string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime.Time}
				}),
			}
		}), nil
	case *appsv1.StatefulSet:
		return base(KindStatefulSet, o, func(r *Resource) {
			r.Workload = &WorkloadSummary{
				Desired: deref(o.Spec.Replicas, 1), Current: o.Status.Replicas, Ready: o.Status.ReadyReplicas,
				Available: o.Status.AvailableReplicas, Updated: o.Status.UpdatedReplicas,
				Generation: o.Generation, ObservedGeneration: o.Status.ObservedGeneration,
				Strategy: string(o.Spec.UpdateStrategy.Type), Selector: selectorOf(o.Spec.Selector),
				Conditions: conditions(o.Status.Conditions, func(c appsv1.StatefulSetCondition) Condition {
					return Condition{string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime.Time}
				}),
			}
		}), nil
	case *appsv1.DaemonSet:
		return base(KindDaemonSet, o, func(r *Resource) {
			r.Workload = &WorkloadSummary{
				Desired: o.Status.DesiredNumberScheduled, Current: o.Status.CurrentNumberScheduled, Ready: o.Status.NumberReady,
				Available: o.Status.NumberAvailable, Updated: o.Status.UpdatedNumberScheduled,
				Generation: o.Generation, ObservedGeneration: o.Status.ObservedGeneration,
				Strategy: string(o.Spec.UpdateStrategy.Type), Selector: selectorOf(o.Spec.Selector),
				Conditions: conditions(o.Status.Conditions, func(c appsv1.DaemonSetCondition) Condition {
					return Condition{string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime.Time}
				}),
			}
		}), nil
	case *batchv1.Job:
		return base(KindJob, o, func(r *Resource) {
			r.Job = &JobSummary{
				Active: o.Status.Active, Succeeded: o.Status.Succeeded, Failed: o.Status.Failed,
				StartTime: timeOf(o.Status.StartTime), CompletionTime: timeOf(o.Status.CompletionTime),
				Conditions: conditions(o.Status.Conditions, func(c batchv1.JobCondition) Condition {
					return Condition{string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime.Time}
				}),
			}
		}), nil
	case *batchv1.CronJob:
		return base(KindCronJob, o, func(r *Resource) {
			r.CronJob = &CronJobSummary{
				Schedule: o.Spec.Schedule, Suspend: deref(o.Spec.Suspend, false), Active: len(o.Status.Active),
				LastScheduleTime: timeOf(o.Status.LastScheduleTime), LastSuccessfulTime: timeOf(o.Status.LastSuccessfulTime),
			}
		}), nil
	case *corev1.Node:
		return base(KindNode, o, func(r *Resource) { r.Node = projectNode(o) }), nil
	case *corev1.Service:
		return base(KindService, o, func(r *Resource) {
			s := &ServiceSummary{Type: string(o.Spec.Type), ClusterIP: o.Spec.ClusterIP, Selector: o.Spec.Selector}
			for _, p := range o.Spec.Ports {
				s.Ports = append(s.Ports, fmt.Sprintf("%s:%d->%s/%s", p.Name, p.Port, p.TargetPort.String(), p.Protocol))
			}
			r.Service = s
		}), nil
	case *corev1.Endpoints:
		return base(KindEndpoints, o, func(r *Resource) { r.Endpoints = projectEndpoints(o) }), nil
	case *networkingv1.Ingress:
		return base(KindIngress, o, func(r *Resource) { r.Ingress = projectIngress(o) }), nil
	default:
		return Resource{}, fmt.Errorf("unsupported object type %T", obj)
	}
}

func base(kind Kind, o metav1.Object, fill func(*Resource)) Resource {
	r := Resource{
		Ref:     Ref{Kind: kind, Name: o.GetName()},
		Labels:  o.GetLabels(),
		Created: o.GetCreationTimestamp().Time,
	}
	if kind.Namespaced() {
		r.Namespace = NamespaceName(o.GetNamespace())
	}
	for _, owner := range o.GetOwnerReferences() {
		r.Owners = append(r.Owners, OwnerRef{Kind: owner.Kind, Name: owner.Name})
	}
	fill(&r)
	return r
}

func projectPod(p *corev1.Pod) *PodSummary {
	s := &PodSummary{
		Phase: string(p.Status.Phase), Reason: p.Status.Reason, Message: p.Status.Message,
		NodeName: p.Spec.NodeName, QOSClass: string(p.Status.QOSClass), StartTime: timeOf(p.Status.StartTime),
		NodeSelector: p.Spec.NodeSelector,
		Conditions: conditions(p.Status.Conditions, func(c corev1.PodCondition) Condition {
			return Condition{string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime.Time}
		}),
		Containers: []ContainerSummary{},
	}
	for _, ref := range p.Spec.ImagePullSecrets {
		s.ImagePullSecrets = append(s.ImagePullSecrets, ref.Name)
	}
	for _, c := range p.Spec.InitContainers {
		s.Containers = append(s.Containers, projectContainer(c, true, statusFor(p.Status.InitContainerStatuses, c.Name)))
	}
	for _, c := range p.Spec.Containers {
		s.Containers = append(s.Containers, projectContainer(c, false, statusFor(p.Status.ContainerStatuses, c.Name)))
	}
	return s
}

func projectContainer(c corev1.Container, init bool, st *corev1.ContainerStatus) ContainerSummary {
	out := ContainerSummary{
		Name: c.Name, Image: c.Image, Init: init,
		Requests: quantities(c.Resources.Requests), Limits: quantities(c.Resources.Limits),
	}
	for _, e := range c.Env {
		out.EnvNames = append(out.EnvNames, e.Name)
	}
	for kind, probe := range map[string]*corev1.Probe{"liveness": c.LivenessProbe, "readiness": c.ReadinessProbe, "startup": c.StartupProbe} {
		if probe != nil {
			out.Probes = append(out.Probes, projectProbe(kind, probe))
		}
	}
	sort.Slice(out.Probes, func(i, j int) bool { return out.Probes[i].Kind < out.Probes[j].Kind })
	if st != nil {
		out.Ready = st.Ready
		out.RestartCount = st.RestartCount
		out.State = containerState(st.State)
		if st.LastTerminationState.Terminated != nil {
			last := containerState(st.LastTerminationState)
			out.LastTerminated = &last
		}
	}
	return out
}

func projectProbe(kind string, p *corev1.Probe) ProbeSummary {
	var handler string
	switch {
	case p.HTTPGet != nil:
		handler = fmt.Sprintf("httpGet %s://%s:%s%s", strings.ToLower(string(p.HTTPGet.Scheme)), p.HTTPGet.Host, p.HTTPGet.Port.String(), p.HTTPGet.Path)
	case p.TCPSocket != nil:
		handler = "tcpSocket " + p.TCPSocket.Port.String()
	case p.GRPC != nil:
		handler = fmt.Sprintf("grpc :%d", p.GRPC.Port)
	case p.Exec != nil:
		handler = "exec"
	}
	return ProbeSummary{
		Kind: kind, Handler: handler, InitialDelaySeconds: p.InitialDelaySeconds, PeriodSeconds: p.PeriodSeconds,
		TimeoutSeconds: p.TimeoutSeconds, FailureThreshold: p.FailureThreshold,
	}
}

func containerState(s corev1.ContainerState) ContainerState {
	switch {
	case s.Terminated != nil:
		t := s.Terminated
		return ContainerState{Phase: "Terminated", Reason: t.Reason, Message: t.Message, ExitCode: t.ExitCode, Signal: t.Signal, StartedAt: t.StartedAt.Time, FinishedAt: t.FinishedAt.Time}
	case s.Waiting != nil:
		return ContainerState{Phase: "Waiting", Reason: s.Waiting.Reason, Message: s.Waiting.Message}
	case s.Running != nil:
		return ContainerState{Phase: "Running", StartedAt: s.Running.StartedAt.Time}
	}
	return ContainerState{}
}

func projectNode(n *corev1.Node) *NodeSummary {
	s := &NodeSummary{
		Unschedulable: n.Spec.Unschedulable, KubeletVersion: n.Status.NodeInfo.KubeletVersion,
		OSImage: n.Status.NodeInfo.OSImage, ContainerRuntime: n.Status.NodeInfo.ContainerRuntimeVersion,
		Capacity: quantities(n.Status.Capacity), Allocatable: quantities(n.Status.Allocatable),
		Conditions: conditions(n.Status.Conditions, func(c corev1.NodeCondition) Condition {
			return Condition{string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime.Time}
		}),
	}
	for _, t := range n.Spec.Taints {
		s.Taints = append(s.Taints, t.ToString())
	}
	return s
}

func projectEndpoints(e *corev1.Endpoints) *EndpointsSummary {
	s := &EndpointsSummary{Ready: []EndpointAddress{}, NotReady: []EndpointAddress{}}
	for _, subset := range e.Subsets {
		s.Ready = append(s.Ready, addresses(subset.Addresses)...)
		s.NotReady = append(s.NotReady, addresses(subset.NotReadyAddresses)...)
	}
	return s
}

func addresses(in []corev1.EndpointAddress) []EndpointAddress {
	out := make([]EndpointAddress, 0, len(in))
	for _, a := range in {
		addr := EndpointAddress{IP: a.IP, NodeName: deref(a.NodeName, "")}
		if a.TargetRef != nil {
			addr.TargetKind, addr.TargetName = a.TargetRef.Kind, a.TargetRef.Name
		}
		out = append(out, addr)
	}
	return out
}

func projectIngress(i *networkingv1.Ingress) *IngressSummary {
	s := &IngressSummary{Class: deref(i.Spec.IngressClassName, "")}
	for _, rule := range i.Spec.Rules {
		if rule.HTTP == nil {
			s.Rules = append(s.Rules, IngressRule{Host: rule.Host, Backend: backendOf(i.Spec.DefaultBackend)})
			continue
		}
		for _, p := range rule.HTTP.Paths {
			s.Rules = append(s.Rules, IngressRule{Host: rule.Host, Path: p.Path, Backend: backendOf(&p.Backend)})
		}
	}
	for _, t := range i.Spec.TLS {
		s.TLSHosts = append(s.TLSHosts, t.Hosts...)
	}
	for _, lb := range i.Status.LoadBalancer.Ingress {
		s.LoadBalancer = append(s.LoadBalancer, lb.Hostname+lb.IP)
	}
	return s
}

func backendOf(b *networkingv1.IngressBackend) string {
	switch {
	case b == nil:
		return ""
	case b.Service != nil:
		return fmt.Sprintf("service/%s:%s%d", b.Service.Name, b.Service.Port.Name, b.Service.Port.Number)
	case b.Resource != nil:
		return b.Resource.Kind + "/" + b.Resource.Name
	}
	return ""
}

func statusFor(statuses []corev1.ContainerStatus, name string) *corev1.ContainerStatus {
	for i := range statuses {
		if statuses[i].Name == name {
			return &statuses[i]
		}
	}
	return nil
}

func conditions[T any](in []T, conv func(T) Condition) []Condition {
	if len(in) == 0 {
		return nil
	}
	out := make([]Condition, 0, len(in))
	for _, c := range in {
		out = append(out, conv(c))
	}
	return out
}

func quantities(list corev1.ResourceList) map[string]string {
	if len(list) == 0 {
		return nil
	}
	out := make(map[string]string, len(list))
	for name, q := range list {
		out[string(name)] = (&q).String()
	}
	return out
}

func selectorOf(s *metav1.LabelSelector) map[string]string {
	if s == nil {
		return nil
	}
	return s.MatchLabels
}

func timeOf(t *metav1.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.Time
}

func deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}
