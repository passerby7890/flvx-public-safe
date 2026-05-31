package handler

import "testing"

func TestReconcileNodeNetworkFields_WithReportedAddresses(t *testing.T) {
	existing := &nodeRecord{
		ServerIP:   "198.51.100.20",
		ServerIPv4: "198.51.100.20",
		ExtraIPs:   "198.51.100.21",
	}
	info := &nodeNetworkInfoSnapshot{
		IPv4: []string{"198.51.100.30", "198.51.100.31"},
		IPv6: []string{"2001:db8::10", "2001:db8::11"},
	}

	serverIP, serverIPv4, serverIPv6, extraIPs := reconcileNodeNetworkFields(existing, info, "198.51.100.30")
	if serverIP != "198.51.100.30" {
		t.Fatalf("expected serverIP to prefer observed/reported IPv4, got %q", serverIP)
	}
	if serverIPv4 != "198.51.100.30" {
		t.Fatalf("expected serverIPv4 to be refreshed, got %q", serverIPv4)
	}
	if serverIPv6 != "2001:db8::10" {
		t.Fatalf("expected serverIPv6 to be refreshed, got %q", serverIPv6)
	}
	if extraIPs != "198.51.100.31,2001:db8::11" {
		t.Fatalf("expected extraIPs to contain remaining addresses, got %q", extraIPs)
	}
}

func TestReconcileNodeNetworkFields_PrefersReportedEntryIPOverObservedControlIP(t *testing.T) {
	existing := &nodeRecord{
		ServerIP:   "198.51.100.105",
		ServerIPv4: "198.51.100.105",
		ServerIPv6: "2001:db8:2002::1058",
		ExtraIPs:   "203.0.113.70,2001:db8:2002::1058,172.16.0.2",
	}
	info := &nodeNetworkInfoSnapshot{
		IPv4: []string{"203.0.113.70", "172.16.0.2"},
		IPv6: []string{"2001:db8:2002::1058", "2001:db8:110::954b"},
	}

	serverIP, serverIPv4, serverIPv6, extraIPs := reconcileNodeNetworkFields(existing, info, "198.51.100.105")
	if serverIP != "203.0.113.70" {
		t.Fatalf("expected reported entry IPv4 to win over observed control IP, got %q", serverIP)
	}
	if serverIPv4 != "203.0.113.70" {
		t.Fatalf("expected serverIPv4 to use reported entry IPv4, got %q", serverIPv4)
	}
	if serverIPv6 != "2001:db8:2002::1058" {
		t.Fatalf("expected serverIPv6 to use reported native IPv6, got %q", serverIPv6)
	}
	if extraIPs != "198.51.100.105,2001:db8:110::954b,172.16.0.2" {
		t.Fatalf("expected extraIPs to retain control-plane and private addresses, got %q", extraIPs)
	}
}

func TestReconcileNodeNetworkFields_PreservesExistingReportedAddressChoice(t *testing.T) {
	existing := &nodeRecord{
		ServerIP:   "203.0.113.20",
		ServerIPv4: "203.0.113.20",
		ServerIPv6: "2001:db8::20",
	}
	info := &nodeNetworkInfoSnapshot{
		IPv4: []string{"203.0.113.10", "203.0.113.20"},
		IPv6: []string{"2001:db8::10", "2001:db8::20"},
	}

	serverIP, serverIPv4, serverIPv6, _ := reconcileNodeNetworkFields(existing, info, "198.51.100.10")
	if serverIP != "203.0.113.20" {
		t.Fatalf("expected existing reported IPv4 choice to stay primary, got %q", serverIP)
	}
	if serverIPv4 != "203.0.113.20" {
		t.Fatalf("expected serverIPv4 to preserve existing reported choice, got %q", serverIPv4)
	}
	if serverIPv6 != "2001:db8::20" {
		t.Fatalf("expected serverIPv6 to preserve existing reported choice, got %q", serverIPv6)
	}
}

func TestReconcileNodeNetworkFields_ObservedFallbackPreservesExistingExtras(t *testing.T) {
	existing := &nodeRecord{
		ServerIP:   "172.237.29.146",
		ServerIPv4: "172.237.29.146",
		ServerIPv6: "2600:1f18::10",
		ExtraIPs:   "172.237.29.147,2600:1f18::11",
	}

	serverIP, serverIPv4, serverIPv6, extraIPs := reconcileNodeNetworkFields(existing, nil, "172.237.29.200")
	if serverIP != "172.237.29.200" {
		t.Fatalf("expected observed ip to refresh legacy serverIP, got %q", serverIP)
	}
	if serverIPv4 != "172.237.29.200" {
		t.Fatalf("expected observed ip to refresh serverIPv4, got %q", serverIPv4)
	}
	if serverIPv6 != "2600:1f18::10" {
		t.Fatalf("expected existing IPv6 to be preserved, got %q", serverIPv6)
	}
	if extraIPs != "172.237.29.147,2600:1f18::11" {
		t.Fatalf("expected existing extraIPs to be preserved, got %q", extraIPs)
	}
}

func TestNormalizeNodeIPList_PublicBeforePrivateAndDeduped(t *testing.T) {
	got := normalizeNodeIPList(
		"10.0.0.5",
		"198.51.100.10",
		"10.0.0.5",
		"2001:db8::1",
		"fd00::1",
	)

	expected := []string{"198.51.100.10", "2001:db8::1", "10.0.0.5", "fd00::1"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d items, got %d (%v)", len(expected), len(got), got)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("expected item %d to be %q, got %q", i, expected[i], got[i])
		}
	}
}

func TestIsGetNetworkInfoUnsupportedError(t *testing.T) {
	if !isGetNetworkInfoUnsupportedError(assertErr("未知命令类型: GetNetworkInfo")) {
		t.Fatal("expected Chinese unsupported error to match")
	}
	if !isGetNetworkInfoUnsupportedError(assertErr("unknown command type: GetNetworkInfo")) {
		t.Fatal("expected English unsupported error to match")
	}
	if isGetNetworkInfoUnsupportedError(assertErr("timeout")) {
		t.Fatal("did not expect timeout to be treated as unsupported")
	}
}

func assertErr(message string) error { return testError(message) }

type testError string

func (e testError) Error() string { return string(e) }
