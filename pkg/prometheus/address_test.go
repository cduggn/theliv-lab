package prometheus

import "testing"

func TestServiceProxyAddress(t *testing.T) {
	for _, tc := range []struct {
		name, scheme, want string
		wantErr            bool
	}{
		{"preserves HTTPS default", "", "https", false},
		{"local HTTP service", "http", "http", false},
		{"explicit HTTPS", "https", "https", false},
		{"rejects invalid scheme", "file", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := serviceProxyAddress("https://127.0.0.1:6443/", "monitoring", "prometheus", "80", tc.scheme)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantErr {
				want := "https://127.0.0.1:6443/api/v1/namespaces/monitoring/services/" + tc.want + ":prometheus:80/proxy"
				if got != want {
					t.Fatalf("got %q; want %q", got, want)
				}
			}
		})
	}
	if _, err := serviceProxyAddress("https://localhost", "", "prometheus", "80", "http"); err == nil {
		t.Fatal("missing namespace must fail")
	}
}
