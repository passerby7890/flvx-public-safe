package handler

import (
	"testing"
)

// ---------------------------------------------------------------------------
// nodeSupportsV4 / nodeSupportsV6
// ---------------------------------------------------------------------------

func TestSelectTunnelDialHost_ConnectIpPriority(t *testing.T) {
	from := dualStackNode("from", "10.0.0.1", "2001:db8::1")
	to := dualStackNode("to", "10.0.0.2", "2001:db8::2")

	// Empty connectIp should be ignored, IP preference takes effect
	host, err := selectTunnelDialHost(from, to, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "10.0.0.2" {
		t.Fatalf("empty connectIp should be ignored (v4 preference applies), got %q", host)
	}
	// Non-empty connectIp should override IP preference
	host, err = selectTunnelDialHost(from, to, "v6", "192.168.0.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "192.168.0.3" {
		t.Fatalf("connectIp should override v6 preference, got %q", host)
	}
}

func TestBuildTunnelChainServiceConfig_UsesConnectIPForListen(t *testing.T) {
	node := &nodeRecord{TCPListenAddr: "[::]"}
	chain := tunnelRuntimeNode{Protocol: "tls", Port: 21000, ConnectIP: "2001:db8::88"}
	services := buildTunnelChainServiceConfig(99, chain, node, 1)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	addr, _ := services[0]["addr"].(string)
	if addr != "[2001:db8::88]:21000" {
		t.Fatalf("expected connectIp listen [2001:db8::88]:21000, got %q", addr)
	}
}

func TestBuildTunnelChainServiceConfig_FallsBackToNodeListenAddr(t *testing.T) {
	node := &nodeRecord{TCPListenAddr: "10.8.0.5"}
	chain := tunnelRuntimeNode{Protocol: "tls", Port: 21002}
	services := buildTunnelChainServiceConfig(99, chain, node, 1)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	addr, _ := services[0]["addr"].(string)
	if addr != "10.8.0.5:21002" {
		t.Fatalf("expected node listen addr 10.8.0.5:21002, got %q", addr)
	}
}

func TestBuildTunnelChainServiceConfig_DefaultListenAddrWhenConnectIPEmpty(t *testing.T) {
	node := &nodeRecord{TCPListenAddr: "[::]"}
	chain := tunnelRuntimeNode{Protocol: "tls", Port: 21001}
	services := buildTunnelChainServiceConfig(99, chain, node, 1)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	addr, _ := services[0]["addr"].(string)
	if addr != "[::]:21001" {
		t.Fatalf("expected default listen [::]:21001, got %q", addr)
	}
}

func TestBuildTunnelChainServiceConfig_WSSUsesSelectedProtocol(t *testing.T) {
	node := &nodeRecord{TCPListenAddr: "[::]"}
	chain := tunnelRuntimeNode{Protocol: "wss", Port: 21004}
	services := buildTunnelChainServiceConfig(99, chain, node, 1)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	listener, _ := services[0]["listener"].(map[string]interface{})
	if listener == nil {
		t.Fatal("expected listener config")
	}
	if listener["type"] != "wss" {
		t.Fatalf("expected listener type wss, got %#v", listener["type"])
	}
	handler, _ := services[0]["handler"].(map[string]interface{})
	if handler == nil {
		t.Fatal("expected handler config")
	}
	metadata, _ := handler["metadata"].(map[string]interface{})
	if metadata == nil || metadata["nodelay"] != true {
		t.Fatalf("expected tls-family handler metadata with nodelay=true, got %#v", metadata)
	}
}

func TestBuildTunnelChainConfig_UsesSelectedProtocolForDialer(t *testing.T) {
	fromNode := &nodeRecord{ID: 1, Name: "from", ServerIPv4: "10.0.0.1"}
	toNode := &nodeRecord{ID: 2, Name: "to", ServerIPv4: "10.0.0.2"}
	cfg, err := buildTunnelChainConfig(99, fromNode.ID, []tunnelRuntimeNode{{
		NodeID:    toNode.ID,
		Protocol:  "mtcp",
		Strategy:  "round",
		ChainType: 3,
		Port:      21005,
	}}, map[int64]*nodeRecord{fromNode.ID: fromNode, toNode.ID: toNode}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hops, _ := cfg["hops"].([]map[string]interface{})
	if len(hops) != 1 {
		t.Fatalf("expected 1 hop, got %d", len(hops))
	}
	nodes, _ := hops[0]["nodes"].([]map[string]interface{})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	dialer, _ := nodes[0]["dialer"].(map[string]interface{})
	if dialer == nil {
		t.Fatal("expected dialer config")
	}
	if dialer["type"] != "mtcp" {
		t.Fatalf("expected dialer type mtcp, got %#v", dialer["type"])
	}
}

func TestNormalizeTunnelProtocol(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{in: "", want: "tls", ok: true},
		{in: "TLS", want: "tls", ok: true},
		{in: "wss", want: "wss", ok: true},
		{in: "tcp", want: "tcp", ok: true},
		{in: "mtls", want: "mtls", ok: true},
		{in: "mwss", want: "mwss", ok: true},
		{in: "mtcp", want: "mtcp", ok: true},
		{in: "quic", want: "", ok: false},
	}

	for _, tt := range tests {
		got, err := normalizeTunnelProtocol(tt.in)
		if tt.ok && err != nil {
			t.Fatalf("expected protocol %q to pass, got error %v", tt.in, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("expected protocol %q to fail", tt.in)
		}
		if tt.ok && got != tt.want {
			t.Fatalf("expected protocol %q -> %q, got %q", tt.in, tt.want, got)
		}
	}
}

func TestIsTLSTunnelProtocol_TLSFamily(t *testing.T) {
	for _, protocol := range []string{"tls", "wss", "mtls", "mwss"} {
		if !isTLSTunnelProtocol(protocol) {
			t.Fatalf("expected %s to be treated as tls-family", protocol)
		}
	}
	for _, protocol := range []string{"tcp", "mtcp"} {
		if isTLSTunnelProtocol(protocol) {
			t.Fatalf("did not expect %s to be treated as tls-family", protocol)
		}
	}
}

func TestBuildTunnelChainServiceConfig_SetsRetriesWhenMultipleCandidates(t *testing.T) {
	node := &nodeRecord{TCPListenAddr: "[::]"}
	chain := tunnelRuntimeNode{Protocol: "tls", Port: 21001}
	services := buildTunnelChainServiceConfig(99, chain, node, 3)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	handler, _ := services[0]["handler"].(map[string]interface{})
	if handler == nil {
		t.Fatal("expected handler config")
	}
	retries, ok := handler["retries"].(int)
	if !ok {
		t.Fatal("expected retries to be set when nextHopCandidateCount > 1")
	}
	if retries != 2 {
		t.Fatalf("expected retries=2 (candidates-1), got %d", retries)
	}
}

func TestBuildTunnelChainServiceConfig_NoRetriesWhenSingleCandidate(t *testing.T) {
	node := &nodeRecord{TCPListenAddr: "[::]"}
	chain := tunnelRuntimeNode{Protocol: "tls", Port: 21001}
	services := buildTunnelChainServiceConfig(99, chain, node, 1)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	handler, _ := services[0]["handler"].(map[string]interface{})
	if handler == nil {
		t.Fatal("expected handler config")
	}
	if _, hasRetries := handler["retries"]; hasRetries {
		t.Fatal("expected no retries when nextHopCandidateCount is 1")
	}
}

func TestNodeSupportsV6_Nil(t *testing.T) {
	if nodeSupportsV6(nil) {
		t.Fatal("nil node must not support v6")
	}
}

func TestNodeSupportsV4_ExplicitV4(t *testing.T) {
	n := &nodeRecord{ServerIPv4: "10.0.0.1"}
	if !nodeSupportsV4(n) {
		t.Fatal("explicit server_ip_v4 needs support v4")
	}
}

func TestNodeSupportsV6_ExplicitV6(t *testing.T) {
	n := &nodeRecord{ServerIPv6: "2001:db8::1"}
	if !nodeSupportsV6(n) {
		t.Fatal("explicit server_ip_v6 needs support v6")
	}
}

func TestNodeSupportsV4_OnlyV6Set(t *testing.T) {
	n := &nodeRecord{ServerIPv6: "2001:db8::1"}
	if nodeSupportsV4(n) {
		t.Fatal("node with only v6 should not support v4")
	}
}

func TestNodeSupportsV6_OnlyV4Set(t *testing.T) {
	n := &nodeRecord{ServerIPv4: "10.0.0.1"}
	if nodeSupportsV6(n) {
		t.Fatal("node with only v4 should not support v6")
	}
}

func TestNodeSupportsV4_DualStack(t *testing.T) {
	n := &nodeRecord{ServerIPv4: "10.0.0.1", ServerIPv6: "2001:db8::1"}
	if !nodeSupportsV4(n) {
		t.Fatal("dual-stack node must support v4")
	}
}

func TestNodeSupportsV6_DualStack(t *testing.T) {
	n := &nodeRecord{ServerIPv4: "10.0.0.1", ServerIPv6: "2001:db8::1"}
	if !nodeSupportsV6(n) {
		t.Fatal("dual-stack node must support v6")
	}
}

func TestNodeSupportsV4_LegacyV4Only(t *testing.T) {
	n := &nodeRecord{ServerIP: "192.168.1.1"}
	if !nodeSupportsV4(n) {
		t.Fatal("legacy v4 ip in server_ip must support v4")
	}
	if nodeSupportsV6(n) {
		t.Fatal("legacy v4 ip in server_ip should not support v6")
	}
}

func TestNodeSupportsV6_LegacyV6Only(t *testing.T) {
	n := &nodeRecord{ServerIP: "2001:db8::1"}
	if !nodeSupportsV6(n) {
		t.Fatal("legacy v6 ip in server_ip must support v6")
	}
	if nodeSupportsV4(n) {
		t.Fatal("legacy v6 ip in server_ip should not support v4")
	}
}

func TestNodeSupportsV4_EmptyNode(t *testing.T) {
	n := &nodeRecord{}
	if nodeSupportsV4(n) {
		t.Fatal("empty node must not support v4")
	}
	if nodeSupportsV6(n) {
		t.Fatal("empty node must not support v6")
	}
}

func TestNodeSupportsV4_LegacyBracketed(t *testing.T) {
	n := &nodeRecord{ServerIP: "[::1]"}
	if nodeSupportsV4(n) {
		t.Fatal("bracketed ipv6 must not support v4")
	}
	if !nodeSupportsV6(n) {
		t.Fatal("bracketed ipv6 must support v6")
	}
}

// ---------------------------------------------------------------------------
// pickNodeAddressV4 / pickNodeAddressV6
// ---------------------------------------------------------------------------

func TestPickNodeAddressV4_Nil(t *testing.T) {
	if pickNodeAddressV4(nil) != "" {
		t.Fatal("nil node must return empty")
	}
}

func TestPickNodeAddressV6_Nil(t *testing.T) {
	if pickNodeAddressV6(nil) != "" {
		t.Fatal("nil node must return empty")
	}
}

func TestPickNodeAddressV4_PreferExplicit(t *testing.T) {
	n := &nodeRecord{ServerIPv4: "10.0.0.1", ServerIP: "192.168.0.1"}
	got := pickNodeAddressV4(n)
	if got != "10.0.0.1" {
		t.Fatalf("expected explicit v4 10.0.0.1, got %q", got)
	}
}

func TestPickNodeAddressV4_FallbackLegacy(t *testing.T) {
	n := &nodeRecord{ServerIP: "192.168.0.1"}
	got := pickNodeAddressV4(n)
	if got != "192.168.0.1" {
		t.Fatalf("expected legacy 192.168.0.1, got %q", got)
	}
}

func TestPickNodeAddressV6_PreferExplicit(t *testing.T) {
	n := &nodeRecord{ServerIPv6: "2001:db8::1", ServerIP: "::1"}
	got := pickNodeAddressV6(n)
	if got != "2001:db8::1" {
		t.Fatalf("expected explicit v6 2001:db8::1, got %q", got)
	}
}

func TestPickNodeAddressV6_FallbackLegacy(t *testing.T) {
	n := &nodeRecord{ServerIP: "::1"}
	got := pickNodeAddressV6(n)
	if got != "::1" {
		t.Fatalf("expected legacy ::1, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// selectTunnelDialHost — core IP preference selection logic
// ---------------------------------------------------------------------------

func dualStackNode(name, v4, v6 string) *nodeRecord {
	return &nodeRecord{
		Name:       name,
		ServerIPv4: v4,
		ServerIPv6: v6,
	}
}

func v4OnlyNode(name, v4 string) *nodeRecord {
	return &nodeRecord{
		Name:       name,
		ServerIPv4: v4,
	}
}

func v6OnlyNode(name, v6 string) *nodeRecord {
	return &nodeRecord{
		Name:       name,
		ServerIPv6: v6,
	}
}

func TestSelectTunnelDialHost_NilNodes(t *testing.T) {
	_, err := selectTunnelDialHost(nil, nil, "", "")
	if err == nil {
		t.Fatal("expected error for nil nodes")
	}
	_, err = selectTunnelDialHost(dualStackNode("a", "1.1.1.1", "::1"), nil, "", "")
	if err == nil {
		t.Fatal("expected error for nil toNode")
	}
	_, err = selectTunnelDialHost(nil, dualStackNode("b", "1.1.1.1", "::1"), "", "")
	if err == nil {
		t.Fatal("expected error for nil fromNode")
	}
}

func TestSelectTunnelDialHost_DualStack_DefaultPreference(t *testing.T) {
	from := dualStackNode("from", "10.0.0.1", "2001:db8::1")
	to := dualStackNode("to", "10.0.0.2", "2001:db8::2")
	host, err := selectTunnelDialHost(from, to, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default prefers v4 when both available
	if host != "10.0.0.2" {
		t.Fatalf("default preference should pick v4, got %q", host)
	}
}

func TestSelectTunnelDialHost_DualStack_PreferV4(t *testing.T) {
	from := dualStackNode("from", "10.0.0.1", "2001:db8::1")
	to := dualStackNode("to", "10.0.0.2", "2001:db8::2")
	host, err := selectTunnelDialHost(from, to, "v4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "10.0.0.2" {
		t.Fatalf("v4 preference should pick v4 address, got %q", host)
	}
}

func TestSelectTunnelDialHost_DualStack_PreferV6(t *testing.T) {
	from := dualStackNode("from", "10.0.0.1", "2001:db8::1")
	to := dualStackNode("to", "10.0.0.2", "2001:db8::2")
	host, err := selectTunnelDialHost(from, to, "v6", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "2001:db8::2" {
		t.Fatalf("v6 preference should pick v6 address, got %q", host)
	}
}

func TestSelectTunnelDialHost_V4Only_PreferV6Fallback(t *testing.T) {
	from := v4OnlyNode("from", "10.0.0.1")
	to := v4OnlyNode("to", "10.0.0.2")
	// User prefers v6, but both nodes are v4-only — should fallback to v4
	host, err := selectTunnelDialHost(from, to, "v6", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "10.0.0.2" {
		t.Fatalf("v6 preference on v4-only nodes should fallback to v4, got %q", host)
	}
}

func TestSelectTunnelDialHost_V6Only_PreferV4Fallback(t *testing.T) {
	from := v6OnlyNode("from", "2001:db8::1")
	to := v6OnlyNode("to", "2001:db8::2")
	// User prefers v4, but both nodes are v6-only — should fallback to v6
	host, err := selectTunnelDialHost(from, to, "v4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "2001:db8::2" {
		t.Fatalf("v4 preference on v6-only nodes should fallback to v6, got %q", host)
	}
}

func TestSelectTunnelDialHost_CrossVersion_V4ToV6(t *testing.T) {
	// v4-only -> v6-only: 跨版本支持，应成功返回 v6 地址
	from := v4OnlyNode("from", "10.0.0.1")
	to := v6OnlyNode("to", "2001:db8::2")
	host, err := selectTunnelDialHost(from, to, "", "")
	if err != nil {
		t.Fatalf("unexpected error for cross-version (v4-only -> v6-only): %v", err)
	}
	if host != "2001:db8::2" {
		t.Fatalf("expected v6 address for cross-version, got %q", host)
	}
}

func TestSelectTunnelDialHost_CrossVersion_V6ToV4(t *testing.T) {
	// v6-only -> v4-only: 跨版本支持，应成功返回 v4 地址
	from := v6OnlyNode("from", "2001:db8::1")
	to := v4OnlyNode("to", "10.0.0.2")
	host, err := selectTunnelDialHost(from, to, "", "")
	if err != nil {
		t.Fatalf("unexpected error for cross-version (v6-only -> v4-only): %v", err)
	}
	if host != "10.0.0.2" {
		t.Fatalf("expected v4 address for cross-version, got %q", host)
	}
}

func TestSelectTunnelDialHost_TrulyIncompatible(t *testing.T) {
	// 真正不兼容：两个节点都没有任何 IP
	from := &nodeRecord{Name: "empty-from", ServerIPv4: "", ServerIPv6: "", ServerIP: ""}
	to := &nodeRecord{Name: "empty-to", ServerIPv4: "", ServerIPv6: "", ServerIP: ""}
	_, err := selectTunnelDialHost(from, to, "", "")
	if err == nil {
		t.Fatal("expected error for nodes with no IP addresses")
	}
}

func TestSelectTunnelDialHost_WhitespacePreference(t *testing.T) {
	from := dualStackNode("from", "10.0.0.1", "2001:db8::1")
	to := dualStackNode("to", "10.0.0.2", "2001:db8::2")
	// Whitespace should be trimmed, treated as "v6"
	host, err := selectTunnelDialHost(from, to, "  v6  ", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "2001:db8::2" {
		t.Fatalf("trimmed v6 preference should pick v6 address, got %q", host)
	}
}

func TestSelectTunnelDialHost_MixedStack_FromDualToV4(t *testing.T) {
	from := dualStackNode("from", "10.0.0.1", "2001:db8::1")
	to := v4OnlyNode("to", "10.0.0.2")
	// v6 preferred, but target only has v4 — should succeed with v4
	host, err := selectTunnelDialHost(from, to, "v6", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "10.0.0.2" {
		t.Fatalf("should fallback to v4 when target is v4-only, got %q", host)
	}
}

func TestSelectTunnelDialHost_MixedStack_FromDualToV6(t *testing.T) {
	from := dualStackNode("from", "10.0.0.1", "2001:db8::1")
	to := v6OnlyNode("to", "2001:db8::2")
	// v4 preferred, but target only has v6 — should succeed with v6
	host, err := selectTunnelDialHost(from, to, "v4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "2001:db8::2" {
		t.Fatalf("should fallback to v6 when target is v6-only, got %q", host)
	}
}

func TestSelectTunnelDialHost_MixedStack_FromV4ToDual(t *testing.T) {
	from := v4OnlyNode("from", "10.0.0.1")
	to := dualStackNode("to", "10.0.0.2", "2001:db8::2")
	// v6 preferred, but from only has v4 — should use v4 (from can only reach v4 of target)
	host, err := selectTunnelDialHost(from, to, "v6", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "10.0.0.2" {
		t.Fatalf("should use v4 when from is v4-only, got %q", host)
	}
}

func TestSelectTunnelDialHost_MixedStack_FromV6ToDual(t *testing.T) {
	from := v6OnlyNode("from", "2001:db8::1")
	to := dualStackNode("to", "10.0.0.2", "2001:db8::2")
	// v4 preferred, but from only has v6 — should use v6
	host, err := selectTunnelDialHost(from, to, "v4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "2001:db8::2" {
		t.Fatalf("should use v6 when from is v6-only, got %q", host)
	}
}

// ---------------------------------------------------------------------------
// nodeDisplayName
// ---------------------------------------------------------------------------

func TestNodeDisplayName_Nil(t *testing.T) {
	got := nodeDisplayName(nil)
	if got != "node" {
		t.Fatalf("nil node display name should be 'node', got %q", got)
	}
}

func TestNodeDisplayName_Named(t *testing.T) {
	n := &nodeRecord{ID: 42, Name: "hk-node"}
	got := nodeDisplayName(n)
	if got != "hk-node" {
		t.Fatalf("expected 'hk-node', got %q", got)
	}
}
func TestNodeDisplayName_Unnamed(t *testing.T) {
	n := &nodeRecord{ID: 42}
	got := nodeDisplayName(n)
	if got != "node_42" {
		t.Fatalf("expected 'node_42', got %q", got)
	}
}
