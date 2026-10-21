package evidence

import "testing"

func TestParseScope(t *testing.T) {
	good, err := ParseScope("rks-local", "theliv-demo")
	if err != nil || good != (Scope{Cluster: "rks-local", Namespace: "theliv-demo"}) {
		t.Fatalf("ParseScope(good) = %v, %v", good, err)
	}
	for _, bad := range [][2]string{
		{"rks-local", "theliv-demo/../other"},
		{"rks-local", ""},
		{"RKS-LOCAL", "theliv-demo"},
		{"rks-local", "Theliv"},
		{"rks local", "theliv-demo"},
	} {
		if _, err := ParseScope(bad[0], bad[1]); err == nil {
			t.Errorf("ParseScope(%q, %q) accepted", bad[0], bad[1])
		}
	}
}

func TestParseRef(t *testing.T) {
	ref, err := ParseRef("pod", "web-0")
	if err != nil || ref != (Ref{Kind: KindPod, Name: "web-0"}) {
		t.Fatalf("ParseRef(pod, web-0) = %v, %v", ref, err)
	}
	for _, bad := range [][2]string{{"Secret", "db"}, {"ConfigMap", "cfg"}, {"Pod", "Web"}, {"Pod", ""}, {"", "x"}} {
		if _, err := ParseRef(bad[0], bad[1]); err == nil {
			t.Errorf("ParseRef(%q, %q) accepted", bad[0], bad[1])
		}
	}
	if KindNode.Namespaced() || !KindPod.Namespaced() {
		t.Error("Namespaced: Node must be cluster scoped and Pod namespaced")
	}
	if n := len(AllKinds()); n != 11 {
		t.Errorf("AllKinds has %d kinds, want 11", n)
	}
}
