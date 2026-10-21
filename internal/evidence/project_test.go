package evidence

import (
	"encoding/json/v2"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func crashingPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-0", Namespace: "theliv-demo",
			Labels:          map[string]string{"app": "web"},
			Annotations:     map[string]string{"kubectl.kubernetes.io/last-applied-configuration": `{"secret":"in-annotation"}`},
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "web-abc"}},
		},
		Spec: corev1.PodSpec{
			NodeName:         "node-1",
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "regcred"}},
			Containers: []corev1.Container{{
				Name: "web", Image: "ghcr.io/x/web:1",
				Env:           []corev1.EnvVar{{Name: "DB_PASSWORD", Value: "hunter2"}},
				Resources:     corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")}},
				LivenessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(8080)}}, PeriodSeconds: 10},
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "web", RestartCount: 3,
				State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: "back-off 5m"}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 137, Reason: "OOMKilled"}},
			}},
		},
	}
}

func TestProjectPodKeepsSignalAndDropsSecrets(t *testing.T) {
	r, err := Project(crashingPod())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	out := string(body)
	for _, leak := range []string{"hunter2", "last-applied", "in-annotation", "managedFields"} {
		if strings.Contains(out, leak) {
			t.Errorf("projection leaked %q in %s", leak, out)
		}
	}
	for _, want := range []string{
		`"ref":{"kind":"Pod","name":"web-0"}`,
		`"owners":[{"kind":"ReplicaSet","name":"web-abc"}]`,
		`"restartCount":3`,
		`"state":{"phase":"Waiting","reason":"CrashLoopBackOff","message":"back-off 5m"}`,
		`"lastTerminated":{"phase":"Terminated","reason":"OOMKilled","exitCode":137}`,
		`"limits":{"memory":"128Mi"}`,
		`"probes":[{"kind":"liveness","handler":"httpGet ://:8080/healthz","periodSeconds":10}]`,
		`"envNames":["DB_PASSWORD"]`,
		`"imagePullSecrets":["regcred"]`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("projection lacks %s in %s", want, out)
		}
	}
	for _, absent := range []string{`"workload"`, `"node"`, `"service"`, `"job"`} {
		if strings.Contains(out, absent) {
			t.Errorf("pod projection carries %s", absent)
		}
	}
}

func TestProjectEveryKind(t *testing.T) {
	two := int32(2)
	meta := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: "theliv-demo", CreationTimestamp: metav1.NewTime(time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC))}
	}
	cases := map[Kind]struct {
		obj  runtime.Object
		want string
	}{
		KindPod:         {crashingPod(), `"phase":"Running"`},
		KindReplicaSet:  {&appsv1.ReplicaSet{ObjectMeta: meta("rs"), Spec: appsv1.ReplicaSetSpec{Replicas: &two}, Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1}}, `"desired":2,"current":0,"ready":1`},
		KindDeployment:  {&appsv1.Deployment{ObjectMeta: meta("dep"), Spec: appsv1.DeploymentSpec{Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}}}, `"strategy":"RollingUpdate","selector":{"app":"web"}`},
		KindStatefulSet: {&appsv1.StatefulSet{ObjectMeta: meta("sts"), Status: appsv1.StatefulSetStatus{ObservedGeneration: 4}}, `"observedGeneration":4`},
		KindDaemonSet:   {&appsv1.DaemonSet{ObjectMeta: meta("ds"), Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 3, NumberReady: 2}}, `"desired":3,"current":0,"ready":2`},
		KindJob:         {&batchv1.Job{ObjectMeta: meta("job"), Status: batchv1.JobStatus{Failed: 1}}, `"failed":1`},
		KindCronJob:     {&batchv1.CronJob{ObjectMeta: meta("cj"), Spec: batchv1.CronJobSpec{Schedule: "*/5 * * * *"}}, `"schedule":"*/5 * * * *"`},
		KindNode:        {&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Spec: corev1.NodeSpec{Unschedulable: true, Taints: []corev1.Taint{{Key: "k", Value: "v", Effect: corev1.TaintEffectNoSchedule}}}, Status: corev1.NodeStatus{Images: []corev1.ContainerImage{{Names: []string{"secret-image"}}}}}, `"unschedulable":true,"taints":["k=v:NoSchedule"]`},
		KindService:     {&corev1.Service{ObjectMeta: meta("svc"), Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Selector: map[string]string{"app": "web"}, Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt32(8080), Protocol: corev1.ProtocolTCP}}}}, `"ports":["http:80->8080/TCP"]`},
		KindEndpoints:   {&corev1.Endpoints{ObjectMeta: meta("svc"), Subsets: []corev1.EndpointSubset{{NotReadyAddresses: []corev1.EndpointAddress{{IP: "10.0.0.5", TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "web-0"}}}}}}, `"notReady":[{"ip":"10.0.0.5","targetKind":"Pod","targetName":"web-0"}]`},
		KindIngress:     {&networkingv1.Ingress{ObjectMeta: meta("ing"), Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "x.example", IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "svc", Port: networkingv1.ServiceBackendPort{Number: 80}}}}}}}}}}}, `"rules":[{"host":"x.example","path":"/","backend":"service/svc:80"}]`},
	}
	for _, kind := range AllKinds() {
		c, ok := cases[kind]
		if !ok {
			t.Fatalf("no projection case for kind %s", kind)
		}
		r, err := Project(c.obj)
		if err != nil {
			t.Fatalf("Project(%s): %v", kind, err)
		}
		if r.Ref.Kind != kind {
			t.Errorf("Project(%s) set kind %s", kind, r.Ref.Kind)
		}
		body, _ := json.Marshal(r)
		if !strings.Contains(string(body), c.want) {
			t.Errorf("Project(%s) lacks %s in %s", kind, c.want, body)
		}
		if kind == KindNode && (strings.Contains(string(body), "secret-image") || strings.Contains(string(body), `"namespace"`)) {
			t.Errorf("node projection leaked images or namespace: %s", body)
		}
	}
	if _, err := Project(&corev1.Secret{}); err == nil {
		t.Error("Project accepted a Secret")
	}
}
