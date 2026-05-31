package handler

import (
	"testing"

	"go-backend/internal/store/model"
)

func TestParseSNIForwardHosts(t *testing.T) {
	hosts, err := parseSNIForwardHosts(`
# comment
hk1.example.com
*.example.net
`)
	if err != nil {
		t.Fatalf("expected wildcard host to be accepted, got %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[1] != "*.example.net" {
		t.Fatalf("unexpected wildcard host normalization: %#v", hosts[1])
	}

	hosts, err = parseSNIForwardHosts(`
hk1.example.com
jp1.example.net,kr1.example.com
`)
	if err != nil {
		t.Fatalf("parseSNIForwardHosts returned error: %v", err)
	}
	if len(hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(hosts))
	}
	if hosts[0] != "hk1.example.com" || hosts[1] != "jp1.example.net" || hosts[2] != "kr1.example.com" {
		t.Fatalf("unexpected parsed hosts: %#v", hosts)
	}
}

func TestDeriveForwardRemoteAddr(t *testing.T) {
	got, err := deriveForwardRemoteAddr(forwardModeSNI, "127.0.0.1:22345\n127.0.0.1:22346", `hk1.example.com`)
	if err != nil {
		t.Fatalf("deriveForwardRemoteAddr returned error: %v", err)
	}
	if got != "127.0.0.1:22345,127.0.0.1:22346" {
		t.Fatalf("expected pass-through remote addr, got %q", got)
	}
}

func TestBuildForwardServiceConfigs_SNI(t *testing.T) {
	forward := &forwardRecord{
		ID:         10, // Hidden port = 20000 + 10 = 20010
		Mode:       forwardModeSNI,
		SniRules:   "hk1.example.com\njp1.example.com",
		RemoteAddr: "8.8.8.8:443",
		Strategy:   "fifo",
		TunnelID:   7,
	}
	node := &nodeRecord{TCPListenAddr: "[::]", UDPListenAddr: "[::]"}
	tunnel := &tunnelRecord{Type: 2}

	services, err := buildForwardServiceConfigs("1_2_0", forward, tunnel, node, 443, "", nil, true)
	if err != nil {
		t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service for SNI hidden direct, got %d", len(services))
	}

	service := services[0]
	if service["name"] != "1_2_0_tcp" {
		t.Fatalf("unexpected service name: %#v", service["name"])
	}
	if service["addr"] != "127.0.0.1:20010" {
		t.Fatalf("unexpected service addr for hidden SNI direct: %#v", service["addr"])
	}

	handler, _ := service["handler"].(map[string]interface{})
	if handler == nil {
		t.Fatal("expected handler config")
	}
	if handler["type"] != "tcp" {
		t.Fatalf("expected tcp handler, got %#v", handler["type"])
	}
	if _, exists := handler["chain"]; !exists {
		t.Fatalf("expected chain on SNI dispatch service, since it is a direct tunnel")
	}
}

func TestBuildSNISharedForwarderNodes_FallbackMarkedAsBackup(t *testing.T) {
	nodes, err := buildSNISharedForwarderNodes([]model.ForwardRecord{
		{
			ID:       1,
			SniRules: "accept.example.com",
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildSNISharedForwarderNodes returned error: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected nodes")
	}
	last := nodes[len(nodes)-1]
	if last["name"] != "sni_fallback" {
		t.Fatalf("expected fallback node at end, got %#v", last["name"])
	}
	metadata, _ := last["metadata"].(map[string]interface{})
	if metadata == nil || metadata["backup"] != true {
		t.Fatalf("expected fallback node to be marked as backup, got %#v", last["metadata"])
	}
}

func TestBuildSNISharedForwarderNodes_CoveredProxyHostsRouteOnlyToDemux(t *testing.T) {
	nodes, err := buildSNISharedForwarderNodes([]model.ForwardRecord{
		{
			ID:       1,
			SniRules: "hk11.example-entry.test\njp11.example-entry.test",
		},
	}, []sniCoverForwardProfile{
		{
			TunnelID:    25,
			Domains:     []string{"*.example-entry.test", "example-entry.test"},
			LocalListen: "127.0.0.1:10443",
		},
	})
	if err != nil {
		t.Fatalf("buildSNISharedForwarderNodes returned error: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected nodes")
	}

	for _, node := range nodes {
		if node["addr"] == "127.0.0.1:10443" {
			t.Fatalf("shared SNI must not route directly to cover nginx, got %#v", node)
		}
	}

	var hkNode map[string]interface{}
	for _, node := range nodes {
		filter, _ := node["filter"].(map[string]interface{})
		if filter != nil && filter["host"] == "hk11.example-entry.test" {
			hkNode = node
			break
		}
	}
	if hkNode == nil {
		t.Fatalf("expected hk11 SNI node, got %#v", nodes)
	}
	if hkNode["addr"] != sniTLSDemuxAddr(1) {
		t.Fatalf("expected covered SNI host to route to TLS demux, got %#v", hkNode["addr"])
	}
}

func TestBuildForwardServiceConfigs_PreservesHostnameTargetsForSNI(t *testing.T) {
	forward := &forwardRecord{
		ID:         11,
		Mode:       forwardModeSNI,
		SniRules:   "hk1.example.com",
		RemoteAddr: "hk-target.example.com:443",
		Strategy:   "fifo",
		TunnelID:   7,
	}
	node := &nodeRecord{TCPListenAddr: "[::]", UDPListenAddr: "[::]"}

	services, err := buildForwardServiceConfigs("1_2_0", forward, nil, node, 443, "", nil, false)
	if err != nil {
		t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service for SNI hidden forward, got %d", len(services))
	}

	forwarder, _ := services[0]["forwarder"].(map[string]interface{})
	if forwarder == nil {
		t.Fatal("expected forwarder config")
	}
	nodes, _ := forwarder["nodes"].([]map[string]interface{})
	if len(nodes) != 1 {
		t.Fatalf("expected one hostname forwarder node, got %d", len(nodes))
	}
	if nodes[0]["addr"] != "hk-target.example.com:443" {
		t.Fatalf("expected hostname target to be preserved, got %#v", nodes)
	}
}
