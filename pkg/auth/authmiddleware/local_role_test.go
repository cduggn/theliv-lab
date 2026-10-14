package authmiddleware

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Exercise the exact role data seeded by the local launcher with the app's matcher.
func TestLocalDemoRole(t *testing.T) {
	data, err := os.ReadFile("../../../dev/local-role.json")
	if err != nil {
		t.Fatal(err)
	}
	var grants []string
	if err := json.Unmarshal(data, &grants); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/theliv-api/v1/clusters/", true},
		{"/theliv-api/v1/clusters", true},
		{"/theliv-api/v1/clusters/rks-local/namespaces/", true},
		{"/theliv-api/v1/detector/rks-local/theliv-demo/detect", true},
		{"/theliv-api/v1/detector/rks-local/theliv-demo/event", true},
		{"/theliv-api/v1/userinfo/", true},
		{"/theliv-api/v1/configinfo/", true},
		{"/theliv-api/v1/rbac/", false},
		{"/theliv-api/v1/register/", false},
		{"/theliv-api/v1/clusters/eks-production/namespaces/", false},
		{"/theliv-api/v1/detector/eks-production/default/detect", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			path := strings.TrimPrefix(strings.TrimSuffix(tc.path, "/"), URLPrefix)
			got, err := checkPattern(grants, []string{path})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("role permits %q = %v; want %v", path, got, tc.want)
			}
		})
	}
}
