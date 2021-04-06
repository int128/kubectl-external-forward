package envoy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/int128/kubectl-external-forward/pkg/tunnel"
)

func TestNewConfig(t *testing.T) {
	t.Run("Tunnel1", func(t *testing.T) {
		tunnels := []tunnel.Tunnel{
			{
				LocalHost:  "0.0.0.0",
				LocalPort:  10080,
				RemoteHost: "www.example.com",
				RemotePort: 80,
			},
		}
		want := `---
static_resources:
  listeners:
    - name: listener_0
      address:
        socket_address:
          address: 0.0.0.0
          port_value: 10080
      filter_chains:
        - filters:
            - name: envoy.filters.network.tcp_proxy
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
                stat_prefix: destination
                cluster: cluster_0
  clusters:
    - name: cluster_0
      connect_timeout: 30s
      type: LOGICAL_DNS
      dns_lookup_family: V4_ONLY
      load_assignment:
        cluster_name: cluster_0
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: www.example.com
                      port_value: 80
`
		got, err := NewConfig(tunnels)
		if err != nil {
			t.Fatalf("error NewConfig: %s", err)
		}
		if got != want {
			t.Errorf("got != want:\n%s", cmp.Diff(got, want))
		}
	})

	t.Run("Tunnel2", func(t *testing.T) {
		tunnels := []tunnel.Tunnel{
			{
				LocalHost:  "0.0.0.0",
				LocalPort:  10080,
				RemoteHost: "www.example.com",
				RemotePort: 80,
			},
			{
				LocalHost:  "127.0.0.1",
				LocalPort:  15432,
				RemoteHost: "db.staging",
				RemotePort: 5432,
			},
		}
		want := `---
static_resources:
  listeners:
    - name: listener_0
      address:
        socket_address:
          address: 0.0.0.0
          port_value: 10080
      filter_chains:
        - filters:
            - name: envoy.filters.network.tcp_proxy
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
                stat_prefix: destination
                cluster: cluster_0
    - name: listener_1
      address:
        socket_address:
          address: 0.0.0.0
          port_value: 15432
      filter_chains:
        - filters:
            - name: envoy.filters.network.tcp_proxy
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
                stat_prefix: destination
                cluster: cluster_1
  clusters:
    - name: cluster_0
      connect_timeout: 30s
      type: LOGICAL_DNS
      dns_lookup_family: V4_ONLY
      load_assignment:
        cluster_name: cluster_0
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: www.example.com
                      port_value: 80
    - name: cluster_1
      connect_timeout: 30s
      type: LOGICAL_DNS
      dns_lookup_family: V4_ONLY
      load_assignment:
        cluster_name: cluster_1
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: db.staging
                      port_value: 5432
`
		got, err := NewConfig(tunnels)
		if err != nil {
			t.Fatalf("error NewConfig: %s", err)
		}
		if got != want {
			t.Errorf("got != want:\n%s", cmp.Diff(got, want))
		}
	})
}
