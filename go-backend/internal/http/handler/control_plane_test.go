package handler

import (
	"errors"
	"reflect"
	"testing"

	"go-backend/internal/store/repo"
)

func TestBuildForwardControlServiceNamesPauseResume(t *testing.T) {
	base := "12_34_56"
	want := []string{base + "_tcp", base + "_udp"}

	for _, command := range []string{"PauseService", "ResumeService"} {
		got := buildForwardControlServiceNames(base, command)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("command %s expected %v, got %v", command, want, got)
		}
	}
}

func TestBuildForwardControlServiceNamesDelete(t *testing.T) {
	base := "12_34_56"
	want := []string{base, base + "_tcp", base + "_udp"}
	got := buildForwardControlServiceNames(base, " DeleteService ")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestBuildForwardServiceBaseCandidates(t *testing.T) {
	got := buildForwardServiceBaseCandidates(12, 34, 56, []int64{56, 78, 90})
	want := []string{"12_34_56", "12_34_78", "12_34_90", "12_34_0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestBuildForwardServiceBaseCandidatesWithZeroPreferred(t *testing.T) {
	got := buildForwardServiceBaseCandidates(12, 34, 0, []int64{78, 0, 90})
	want := []string{"12_34_0", "12_34_78", "12_34_90"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestAppendLegacyForwardPortServiceBases(t *testing.T) {
	got := appendLegacyForwardPortServiceBases([]string{"12_34_56", "12_34_0"}, 44287)
	want := []string{"12_34_56", "12_34_0", "manual_44287"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestAppendLegacyForwardPortServiceBasesSkipsInvalidPort(t *testing.T) {
	got := appendLegacyForwardPortServiceBases([]string{"12_34_56"}, 0)
	want := []string{"12_34_56"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestBuildForwardServiceBaseWithResolvedUserTunnel(t *testing.T) {
	got := buildForwardServiceBaseWithResolvedUserTunnel(12, 34, 56)
	if got != "12_34_56" {
		t.Fatalf("expected 12_34_56, got %s", got)
	}
}

func TestBuildForwardServiceBaseWithResolvedUserTunnelFallbackToZero(t *testing.T) {
	got := buildForwardServiceBaseWithResolvedUserTunnel(12, 34, 0)
	if got != "12_34_0" {
		t.Fatalf("expected 12_34_0, got %s", got)
	}
}

func TestShouldTryLegacySingleService(t *testing.T) {
	if !shouldTryLegacySingleService("PauseService") {
		t.Fatalf("PauseService should require legacy fallback")
	}
	if !shouldTryLegacySingleService("resumeService") {
		t.Fatalf("ResumeService should require legacy fallback")
	}
	if shouldTryLegacySingleService("DeleteService") {
		t.Fatalf("DeleteService should not require legacy fallback")
	}
}

func TestShouldSelfHealForwardServiceControl(t *testing.T) {
	if !shouldSelfHealForwardServiceControl("PauseService") {
		t.Fatalf("PauseService should trigger self-heal")
	}
	if !shouldSelfHealForwardServiceControl(" resumeService ") {
		t.Fatalf("ResumeService should trigger self-heal")
	}
	if shouldSelfHealForwardServiceControl("DeleteService") {
		t.Fatalf("DeleteService should not trigger self-heal")
	}
}

func TestControlForwardServiceCommandHandledOnKnownVariant(t *testing.T) {
	bases := []string{"12_34_56"}
	called := make([]string, 0)
	handled, lastNotFoundErr, err := controlForwardServiceCommand(bases, "PauseService", func(name string) error {
		called = append(called, name)
		if name == "12_34_56_udp" {
			return nil
		}
		return errors.New("service " + name + " not found")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if lastNotFoundErr != nil {
		t.Fatalf("expected lastNotFoundErr=nil when handled")
	}
	wantCalls := []string{"12_34_56_tcp", "12_34_56_udp", "12_34_56"}
	if !reflect.DeepEqual(called, wantCalls) {
		t.Fatalf("expected calls %v, got %v", wantCalls, called)
	}
}

func TestControlForwardServiceCommandReturnsLastNotFoundWhenAllMissing(t *testing.T) {
	bases := []string{"12_34_56"}
	handled, lastNotFoundErr, err := controlForwardServiceCommand(bases, "PauseService", func(name string) error {
		return errors.New("service " + name + " not found")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatalf("expected handled=false")
	}
	if lastNotFoundErr == nil {
		t.Fatalf("expected lastNotFoundErr when all variants are missing")
	}
}

func TestDeleteForwardServiceCandidatesSkipsNotFoundUntilLegacyMatch(t *testing.T) {
	bases := []string{"12_34_56", "12_34_0"}
	called := make([]string, 0)
	err := deleteForwardServiceCandidates(bases, func(name string) error {
		called = append(called, name)
		if name == "12_34_0" {
			return nil
		}
		return errors.New("service " + name + " not found")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantCalls := []string{"12_34_56_tcp", "12_34_56_udp", "12_34_56", "12_34_0_tcp", "12_34_0_udp", "12_34_0"}
	if !reflect.DeepEqual(called, wantCalls) {
		t.Fatalf("expected calls %v, got %v", wantCalls, called)
	}
}

func TestDeleteForwardServiceCandidatesTreatsAllMissingAsSuccess(t *testing.T) {
	bases := []string{"12_34_56", "12_34_0"}
	err := deleteForwardServiceCandidates(bases, func(name string) error {
		return errors.New("service " + name + " not found")
	})
	if err != nil {
		t.Fatalf("all-missing delete should be tolerated, got %v", err)
	}
}

func TestForwardServiceBaseCandidatesIncludesResolvedAndLegacyZero(t *testing.T) {
	bases := buildForwardServiceBaseCandidates(46, 9, 123, []int64{123, 77, 0})
	want := []string{"46_9_123", "46_9_77", "46_9_0"}
	if !reflect.DeepEqual(bases, want) {
		t.Fatalf("expected %v, got %v", want, bases)
	}
}

func TestDeleteForwardServiceBasesOnNodeRetriesLegacyZeroResidue(t *testing.T) {
	bases := []string{"46_9_123", "46_9_0"}
	called := make([]string, 0)
	err := deleteForwardServiceCandidates(bases, func(name string) error {
		called = append(called, name)
		if name == "46_9_0_tcp" || name == "46_9_0_udp" {
			return nil
		}
		return errors.New("service " + name + " not found")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"46_9_123_tcp", "46_9_123_udp", "46_9_123", "46_9_0_tcp", "46_9_0_udp", "46_9_0"}
	if !reflect.DeepEqual(called, want) {
		t.Fatalf("expected calls %v, got %v", want, called)
	}
}

func TestDeleteForwardServiceCandidatesDeletesAllMatchingVariants(t *testing.T) {
	bases := []string{"57_7_7", "57_7_0"}
	called := make([]string, 0)
	err := deleteForwardServiceCandidates(bases, func(name string) error {
		called = append(called, name)
		switch name {
		case "57_7_7_tcp", "57_7_7_udp", "57_7_0_tcp", "57_7_0_udp":
			return nil
		default:
			return errors.New("service " + name + " not found")
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"57_7_7_tcp", "57_7_7_udp", "57_7_7", "57_7_0_tcp", "57_7_0_udp", "57_7_0"}
	if !reflect.DeepEqual(called, want) {
		t.Fatalf("expected calls %v, got %v", want, called)
	}
}

func TestValidateForwardPortAvailabilityRejectsOtherForwardOccupancy(t *testing.T) {
	h := &Handler{repo: nil}
	node := &nodeRecord{ID: 9, Name: "test-node"}
	_ = h
	_ = node

	rawRepo, err := repo.Open(":memory:")
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	h = &Handler{repo: rawRepo}
	if err := rawRepo.DB().Exec(`INSERT INTO forward(id, user_id, user_name, name, tunnel_id, mode, remote_addr, sni_rules, strategy, in_flow, out_flow, created_time, updated_time, status, inx) VALUES(1, 1, 'user', 'forward-1', 1, 'direct', '127.0.0.1:1', '', 'fifo', 0, 0, 0, 0, 1, 0)`).Error; err != nil {
		t.Fatalf("insert forward: %v", err)
	}
	if err := rawRepo.DB().Exec(`INSERT INTO forward_port(forward_id, node_id, port) VALUES(1, 9, 2000)`).Error; err != nil {
		t.Fatalf("insert forward port: %v", err)
	}

	err = h.validateForwardPortAvailability(&nodeRecord{ID: 9, Name: "test-node"}, 2000, 2, forwardModeDirect, "")
	if err == nil {
		t.Fatalf("expected occupancy error")
	}
	if err.Error() != "节点 test-node 端口 2000 已被其他转发占用" {
		t.Fatalf("unexpected error: %v", err)
	}

	err = h.validateForwardPortAvailability(&nodeRecord{ID: 9, Name: "test-node"}, 2000, 1, forwardModeDirect, "")
	if err != nil {
		t.Fatalf("same forward should be allowed, got %v", err)
	}
}

func TestValidateForwardPortAvailabilityAllowsSharedSNI(t *testing.T) {
	rawRepo, err := repo.Open(":memory:")
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	h := &Handler{repo: rawRepo}

	if err := rawRepo.DB().Exec(`INSERT INTO forward(id, user_id, user_name, name, tunnel_id, mode, remote_addr, sni_rules, strategy, in_flow, out_flow, created_time, updated_time, status, inx) VALUES(1, 1, 'user', 'forward-1', 1, 'sni', '127.0.0.1:1', 'hk1.example.com', 'fifo', 0, 0, 0, 0, 1, 0)`).Error; err != nil {
		t.Fatalf("insert forward: %v", err)
	}
	if err := rawRepo.DB().Exec(`INSERT INTO forward_port(forward_id, node_id, port, in_ip) VALUES(1, 9, 443, '10.0.0.1')`).Error; err != nil {
		t.Fatalf("insert forward port: %v", err)
	}

	if err := h.validateForwardPortAvailability(&nodeRecord{ID: 9, Name: "test-node"}, 443, 2, forwardModeSNI, "10.0.0.1"); err != nil {
		t.Fatalf("shared SNI port should be allowed, got %v", err)
	}

	err = h.validateForwardPortAvailability(&nodeRecord{ID: 9, Name: "test-node"}, 443, 2, forwardModeSNI, "")
	if err == nil {
		t.Fatalf("expected bind IP mismatch error")
	}

	err = h.validateForwardPortAvailability(&nodeRecord{ID: 9, Name: "test-node"}, 443, 2, forwardModeDirect, "")
	if err == nil {
		t.Fatalf("expected direct occupancy error")
	}
}

func TestControlForwardServiceCommandReturnsHardError(t *testing.T) {
	bases := []string{"12_34_56"}
	handled, lastNotFoundErr, err := controlForwardServiceCommand(bases, "PauseService", func(name string) error {
		if name == "12_34_56_tcp" {
			return errors.New("network timeout")
		}
		return nil
	})
	if err == nil {
		t.Fatalf("expected hard error")
	}
	if handled {
		t.Fatalf("expected handled=false on hard error")
	}
	if lastNotFoundErr != nil {
		t.Fatalf("did not expect not-found error alongside hard error")
	}
}

func TestIsAlreadyExistsMessage(t *testing.T) {
	if !isAlreadyExistsMessage("service demo already exists") {
		t.Fatalf("expected already exists message to be tolerated")
	}
	if !isAlreadyExistsMessage("服务已存在") {
		t.Fatalf("expected Chinese already exists message to be tolerated")
	}
	if !isAlreadyExistsMessage("service demo alreadyexists") {
		t.Fatalf("missing-space alreadyexists should be tolerated")
	}
	if isAlreadyExistsMessage("listen tcp [::]:10001: bind: address already in use") {
		t.Fatalf("address already in use must not be treated as already exists")
	}
	if isAlreadyExistsMessage("create service 57_7_7_tcp failed: listen tcp4 0.0.0.0:46222: bind: address alreadyin use") {
		t.Fatalf("alreadyin-use variant must not be treated as already exists")
	}
}

func TestIsBindAddressInUseError(t *testing.T) {
	if !isBindAddressInUseError(errors.New("listen tcp [::]:10001: bind: address already in use")) {
		t.Fatalf("address already in use should be detected")
	}
	if !isBindAddressInUseError(errors.New("listen tcp4 13.228.170.187:16765: bind: cannot assign requested address")) {
		t.Fatalf("cannot assign requested address should be detected")
	}
	if isBindAddressInUseError(errors.New("service demo already exists")) {
		t.Fatalf("already exists should not be treated as bind conflict")
	}
	if isBindAddressInUseError(nil) {
		t.Fatalf("nil error should not be treated as bind conflict")
	}
}

func TestIsAddressAlreadyInUseError(t *testing.T) {
	if !isAddressAlreadyInUseError(errors.New("listen tcp [::]:10001: bind: address already in use")) {
		t.Fatalf("address already in use should be detected")
	}
	if !isAddressAlreadyInUseError(errors.New("create service 57_7_7_tcp failed: listen tcp4 0.0.0.0:46222: bind: address alreadyin use")) {
		t.Fatalf("missing-space alreadyin-use variant should be detected")
	}
	if isAddressAlreadyInUseError(errors.New("listen tcp4 13.228.170.187:16765: bind: cannot assign requested address")) {
		t.Fatalf("cannot assign requested address should not be treated as address-in-use")
	}
}

func TestIsCannotAssignRequestedAddressError(t *testing.T) {
	if !isCannotAssignRequestedAddressError(errors.New("listen tcp4 13.228.170.187:16765: bind: cannot assign requested address")) {
		t.Fatalf("cannot assign requested address should be detected")
	}
	if !isCannotAssignRequestedAddressError(errors.New("listen tcp4 13.228.170.187:16765: bind: cannotassignrequestedaddress")) {
		t.Fatalf("missing-space cannotassignrequestedaddress variant should be detected")
	}
	if isCannotAssignRequestedAddressError(errors.New("listen tcp [::]:10001: bind: address already in use")) {
		t.Fatalf("address already in use should not be treated as cannot-assign")
	}
}

func TestRetryTunnelServiceAddWithCleanupRetriesOnAddressInUse(t *testing.T) {
	addCalls := 0
	cleanupCalls := 0
	err := retryTunnelServiceAddWithCleanup(
		func() error {
			addCalls++
			if addCalls == 1 {
				return errors.New("listen tcp 10.0.0.1:32000: bind: address already in use")
			}
			return nil
		},
		func() error {
			cleanupCalls++
			return nil
		},
		0,
	)
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if addCalls != 2 {
		t.Fatalf("expected 2 add attempts, got %d", addCalls)
	}
	if cleanupCalls != 1 {
		t.Fatalf("expected 1 cleanup attempt, got %d", cleanupCalls)
	}
}

func TestRetryTunnelServiceAddWithCleanupSkipsCleanupOnNonBindError(t *testing.T) {
	addCalls := 0
	cleanupCalls := 0
	err := retryTunnelServiceAddWithCleanup(
		func() error {
			addCalls++
			return errors.New("network timeout")
		},
		func() error {
			cleanupCalls++
			return nil
		},
		0,
	)
	if err == nil {
		t.Fatalf("expected hard error")
	}
	if addCalls != 1 {
		t.Fatalf("expected 1 add attempt, got %d", addCalls)
	}
	if cleanupCalls != 0 {
		t.Fatalf("expected 0 cleanup attempts, got %d", cleanupCalls)
	}
}

func TestRetryTunnelServiceAddWithCleanupReturnsCleanupError(t *testing.T) {
	cleanupErr := errors.New("delete failed")
	err := retryTunnelServiceAddWithCleanup(
		func() error {
			return errors.New("listen tcp 10.0.0.1:32000: bind: address already in use")
		},
		func() error {
			return cleanupErr
		},
		0,
	)
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("expected cleanup error %v, got %v", cleanupErr, err)
	}
}

func TestRetryServiceAddWithCleanupOnBindConflictRetriesMultipleTimes(t *testing.T) {
	addCalls := 0
	cleanupCalls := 0

	err := retryServiceAddWithCleanupOnBindConflict(
		func() error {
			addCalls++
			if addCalls < 4 {
				return errors.New("listen tcp 10.0.0.1:32000: bind: address already in use")
			}
			return nil
		},
		func() error {
			cleanupCalls++
			return nil
		},
		0,
		0,
		0,
	)
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if addCalls != 4 {
		t.Fatalf("expected 4 add attempts, got %d", addCalls)
	}
	if cleanupCalls != 3 {
		t.Fatalf("expected 3 cleanup attempts, got %d", cleanupCalls)
	}
}

func TestRetryServiceAddWithCleanupOnBindConflictStopsOnNonBindError(t *testing.T) {
	addCalls := 0
	cleanupCalls := 0

	err := retryServiceAddWithCleanupOnBindConflict(
		func() error {
			addCalls++
			if addCalls == 1 {
				return errors.New("listen tcp 10.0.0.1:32000: bind: address already in use")
			}
			return errors.New("network timeout")
		},
		func() error {
			cleanupCalls++
			return nil
		},
		0,
		0,
	)
	if err == nil {
		t.Fatal("expected non-bind retry error")
	}
	if addCalls != 2 {
		t.Fatalf("expected 2 add attempts, got %d", addCalls)
	}
	if cleanupCalls != 1 {
		t.Fatalf("expected 1 cleanup attempt, got %d", cleanupCalls)
	}
}

func TestBuildForwardServiceConfigs_UsesBindIPForListen(t *testing.T) {
	forward := &forwardRecord{RemoteAddr: "1.2.3.4:80", Strategy: "fifo", TunnelID: 7}
	node := &nodeRecord{TCPListenAddr: "[::]", UDPListenAddr: "[::]"}
	services, err := buildForwardServiceConfigs("1_2_0", forward, nil, node, 22000, "10.9.8.7", nil, false)
	if err != nil {
		t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	for _, svc := range services {
		addr, _ := svc["addr"].(string)
		if addr != "10.9.8.7:22000" {
			t.Fatalf("expected bind IP address 10.9.8.7:22000, got %q", addr)
		}
	}
}

func TestBuildForwardServiceConfigs_DefaultListenAddrWhenBindIPEmpty(t *testing.T) {
	forward := &forwardRecord{RemoteAddr: "1.2.3.4:80", Strategy: "fifo", TunnelID: 7}
	node := &nodeRecord{TCPListenAddr: "0.0.0.0", UDPListenAddr: "[::]"}
	services, err := buildForwardServiceConfigs("1_2_0", forward, nil, node, 22001, "", nil, false)
	if err != nil {
		t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	tcpAddr, _ := services[0]["addr"].(string)
	udpAddr, _ := services[1]["addr"].(string)
	if tcpAddr != "0.0.0.0:22001" {
		t.Fatalf("expected tcp addr 0.0.0.0:22001, got %q", tcpAddr)
	}
	if udpAddr != "[::]:22001" {
		t.Fatalf("expected udp addr [::]:22001, got %q", udpAddr)
	}
}

func TestBuildForwardServiceConfigs_TLSFamilySetsUDPTTL(t *testing.T) {
	forward := &forwardRecord{RemoteAddr: "1.2.3.4:80", Strategy: "fifo", TunnelID: 7}
	node := &nodeRecord{TCPListenAddr: "[::]", UDPListenAddr: "[::]"}
	services, err := buildForwardServiceConfigs("1_2_0", forward, nil, node, 22002, "", nil, true)
	if err != nil {
		t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	listener, _ := services[1]["listener"].(map[string]interface{})
	if listener == nil {
		t.Fatal("expected udp listener config")
	}
	metadata, _ := listener["metadata"].(map[string]interface{})
	if metadata == nil {
		t.Fatal("expected udp listener metadata")
	}
	if metadata["ttl"] != "10s" {
		t.Fatalf("expected udp ttl=10s for tls-family tunnel, got %#v", metadata["ttl"])
	}
}

func TestBuildForwardServiceConfigs_NonTLSFamilyDoesNotSetUDPTTL(t *testing.T) {
	forward := &forwardRecord{RemoteAddr: "1.2.3.4:80", Strategy: "fifo", TunnelID: 7}
	node := &nodeRecord{TCPListenAddr: "[::]", UDPListenAddr: "[::]"}
	services, err := buildForwardServiceConfigs("1_2_0", forward, nil, node, 22003, "", nil, false)
	if err != nil {
		t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	listener, _ := services[1]["listener"].(map[string]interface{})
	if listener == nil {
		t.Fatal("expected udp listener config")
	}
	metadata, _ := listener["metadata"].(map[string]interface{})
	if metadata == nil {
		t.Fatal("expected udp listener metadata")
	}
	if _, exists := metadata["ttl"]; exists {
		t.Fatalf("did not expect udp ttl for non-tls-family tunnel, got %#v", metadata["ttl"])
	}
}

func TestBuildForwardServiceConfigs_SkipsInterfaceMetadataForLoopbackTargets(t *testing.T) {
	forward := &forwardRecord{RemoteAddr: "127.0.0.1:9443", Strategy: "fifo", TunnelID: 7}
	tunnel := &tunnelRecord{Type: 1}
	node := &nodeRecord{TCPListenAddr: "0.0.0.0", UDPListenAddr: "[::]", InterfaceName: "eth0"}

	services, err := buildForwardServiceConfigs("1_2_0", forward, tunnel, node, 8443, "", nil, false)
	if err != nil {
		t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
	}
	if len(services) == 0 {
		t.Fatal("expected at least one service")
	}
	if metadata, _ := services[0]["metadata"].(map[string]interface{}); metadata != nil {
		t.Fatalf("did not expect interface metadata for loopback target, got %#v", metadata)
	}
}

func TestBuildForwardServiceConfigs_KeepsInterfaceMetadataForExternalTargets(t *testing.T) {
	forward := &forwardRecord{RemoteAddr: "203.0.113.10:9443", Strategy: "fifo", TunnelID: 7}
	tunnel := &tunnelRecord{Type: 1}
	node := &nodeRecord{TCPListenAddr: "0.0.0.0", UDPListenAddr: "[::]", InterfaceName: "eth0"}

	services, err := buildForwardServiceConfigs("1_2_0", forward, tunnel, node, 8443, "", nil, false)
	if err != nil {
		t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
	}
	if len(services) == 0 {
		t.Fatal("expected at least one service")
	}
	metadata, _ := services[0]["metadata"].(map[string]interface{})
	if metadata == nil {
		t.Fatal("expected interface metadata for external target")
	}
	if metadata["interface"] != "eth0" {
		t.Fatalf("expected interface metadata eth0, got %#v", metadata["interface"])
	}
}

func TestBuildForwardServiceConfigs_BindIPAlreadyContainsPort(t *testing.T) {
	forward := &forwardRecord{RemoteAddr: "1.2.3.4:80", Strategy: "fifo", TunnelID: 7}
	node := &nodeRecord{TCPListenAddr: "[::]", UDPListenAddr: "[::]"}
	services, err := buildForwardServiceConfigs("1_2_0", forward, nil, node, 55555, "3.3.3.3:12345", nil, false)
	if err != nil {
		t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	for _, svc := range services {
		addr, _ := svc["addr"].(string)
		if addr != "3.3.3.3:12345" {
			t.Fatalf("expected bind IP with port 3.3.3.3:12345, got %q", addr)
		}
	}
}

func TestBuildForwardServiceConfigs_IPv6BindIP(t *testing.T) {
	tests := []struct {
		name     string
		bindIP   string
		port     int
		wantAddr string
	}{
		{
			name:     "pure ipv6 without port",
			bindIP:   "2001:db8::1",
			port:     22000,
			wantAddr: "[2001:db8::1]:22000",
		},
		{
			name:     "bracketed ipv6 without port",
			bindIP:   "[2001:db8::2]",
			port:     22001,
			wantAddr: "[2001:db8::2]:22001",
		},
		{
			name:     "bracketed ipv6 with port",
			bindIP:   "[2001:db8::3]:8080",
			port:     55555,
			wantAddr: "[2001:db8::3]:8080",
		},
		{
			name:     "ipv6 link-local with zone",
			bindIP:   "fe80::1%eth0",
			port:     22002,
			wantAddr: "[fe80::1%eth0]:22002",
		},
		{
			name:     "ipv6 localhost",
			bindIP:   "::1",
			port:     22003,
			wantAddr: "[::1]:22003",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forward := &forwardRecord{RemoteAddr: "1.2.3.4:80", Strategy: "fifo", TunnelID: 7}
			node := &nodeRecord{TCPListenAddr: "[::]", UDPListenAddr: "[::]"}
			services, err := buildForwardServiceConfigs("1_2_0", forward, nil, node, tt.port, tt.bindIP, nil, false)
			if err != nil {
				t.Fatalf("buildForwardServiceConfigs returned error: %v", err)
			}
			if len(services) != 2 {
				t.Fatalf("expected 2 services, got %d", len(services))
			}
			for _, svc := range services {
				addr, _ := svc["addr"].(string)
				if addr != tt.wantAddr {
					t.Fatalf("expected addr %q, got %q", tt.wantAddr, addr)
				}
			}
		})
	}
}

func TestBuildForwarderNodesPreservesHostnameTargets(t *testing.T) {
	nodes := buildForwarderNodes([]string{"hk1.example.com:443"})
	if len(nodes) != 1 {
		t.Fatalf("expected single hostname node, got %d", len(nodes))
	}
	addr, _ := nodes[0]["addr"].(string)
	if addr != "hk1.example.com:443" {
		t.Fatalf("expected hostname target to be preserved, got %q", addr)
	}
}

func TestBuildForwarderNodesDeduplicatesTargets(t *testing.T) {
	nodes := buildForwarderNodes([]string{"hk1.example.com:443", "hk1.example.com:443", "203.0.113.8:443"})
	if len(nodes) != 2 {
		t.Fatalf("expected duplicate target removed, got %d nodes", len(nodes))
	}
	if nodes[0]["addr"] != "hk1.example.com:443" || nodes[1]["addr"] != "203.0.113.8:443" {
		t.Fatalf("unexpected nodes: %#v", nodes)
	}
}

func TestProcessServerAddress_StripsURLSchemeAndPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "https with path",
			in:   "https://panel.example.com:8443/api/v1",
			want: "panel.example.com:8443",
		},
		{
			name: "wss with query",
			in:   "wss://panel.example.com:443/system-info?x=1",
			want: "panel.example.com:443",
		},
		{
			name: "http without port",
			in:   "http://panel.example.com",
			want: "panel.example.com",
		},
		{
			name: "manual host with trailing path",
			in:   "panel.example.com:8080/path",
			want: "panel.example.com:8080",
		},
	}

	for _, tt := range tests {
		if got := processServerAddress(tt.in); got != tt.want {
			t.Fatalf("%s: expected %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestProcessServerAddress_NormalizesIPv6(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "ipv6 host only",
			in:   "2001:db8::1",
			want: "[2001:db8::1]",
		},
		{
			name: "ipv6 host and port",
			in:   "https://[2001:db8::1]:8443/path",
			want: "[2001:db8::1]:8443",
		},
		{
			name: "already bracketed",
			in:   "[2001:db8::2]:9000",
			want: "[2001:db8::2]:9000",
		},
	}

	for _, tt := range tests {
		if got := processServerAddress(tt.in); got != tt.want {
			t.Fatalf("%s: expected %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestExpectedAgentListenerRecognizesSSOutput(t *testing.T) {
	info := map[string]interface{}{
		"ss": "State  Recv-Q Send-Q Local Address:Port  Peer Address:PortProcess\n" +
			`LISTEN 0 633 *:58356 *:* users:(("flux_agent",pid=1160,fd=315))`,
	}
	if !expectedAgentListener(info) {
		t.Fatalf("expected ss output mentioning flux_agent to count as agent listener")
	}
}

func TestShouldEstimateEntrypointStatusFromReachabilityWhenPortToolsFail(t *testing.T) {
	info := map[string]interface{}{
		"ss":   `ss [-ltnp sport = :58356] failed: fork/exec /usr/bin/ss: resource temporarily unavailable`,
		"lsof": `lsof [-nP -iTCP:58356 -sTCP:LISTEN] failed: fork/exec /usr/bin/lsof: resource temporarily unavailable`,
	}
	if !shouldEstimateEntrypointStatusFromReachability(info, nil) {
		t.Fatalf("expected reachability fallback when both inspection commands fail")
	}
}

func TestShouldNotEstimateEntrypointStatusFromReachabilityWhenSSSucceeds(t *testing.T) {
	info := map[string]interface{}{
		"ss": "State  Recv-Q Send-Q Local Address:Port  Peer Address:PortProcess\n" +
			`LISTEN 0 633 *:58356 *:* users:(("flux_agent",pid=1160,fd=315))`,
		"lsof": `lsof [-nP -iTCP:58356 -sTCP:LISTEN] failed: exec: "lsof": executable file not found in $PATH`,
	}
	if shouldEstimateEntrypointStatusFromReachability(info, nil) {
		t.Fatalf("did not expect fallback when ss already confirmed the listener")
	}
}

func TestCheckPortFailureReasonPrefersFailedCommandOutput(t *testing.T) {
	info := map[string]interface{}{
		"ss":   `ss [-ltnp sport = :58356] failed: fork/exec /usr/bin/ss: resource temporarily unavailable`,
		"lsof": `lsof [-nP -iTCP:58356 -sTCP:LISTEN] failed: fork/exec /usr/bin/lsof: resource temporarily unavailable`,
	}
	got := checkPortFailureReason(info, nil)
	if got == "" {
		t.Fatalf("expected non-empty probe failure reason")
	}
	if !isCheckPortCommandFailure(got) {
		t.Fatalf("expected probe failure reason, got %q", got)
	}
}

func TestShouldAutoRepairForwardEntryStatus(t *testing.T) {
	tests := []struct {
		name string
		item forwardEntryStatusItem
		want bool
	}{
		{
			name: "healthy entry",
			item: forwardEntryStatusItem{Healthy: true},
			want: false,
		},
		{
			name: "external occupant",
			item: forwardEntryStatusItem{OccupiedByExternal: true, Reason: "/usr/local/bin/V2bX server -c /etc/V2bX/config.json"},
			want: false,
		},
		{
			name: "probe command failure",
			item: forwardEntryStatusItem{Reason: `ss [-ltnp sport = :58356] failed: fork/exec /usr/bin/ss: resource temporarily unavailable`},
			want: false,
		},
		{
			name: "reachability fallback failure",
			item: forwardEntryStatusItem{Reason: "节点未升级，且入口端口不可达"},
			want: false,
		},
		{
			name: "missing listener should self-heal",
			item: forwardEntryStatusItem{Reason: "节点未监听该端口"},
			want: true,
		},
	}

	for _, tt := range tests {
		if got := shouldAutoRepairForwardEntryStatus(tt.item); got != tt.want {
			t.Fatalf("%s: expected %v, got %v", tt.name, tt.want, got)
		}
	}
}

func TestShouldUseTLSApplicationProbe(t *testing.T) {
	if !shouldUseTLSApplicationProbe(forwardModeSNI) {
		t.Fatalf("expected SNI forward to require TLS application probe")
	}
	if shouldUseTLSApplicationProbe(forwardModeDirect) {
		t.Fatalf("expected direct forward to skip TLS application probe")
	}
	if shouldUseTLSApplicationProbe("") {
		t.Fatalf("expected default direct forward mode to skip TLS application probe")
	}
}

func TestForwardEntryTLSProbeServerName(t *testing.T) {
	forward := &forwardRecord{
		Mode:     forwardModeSNI,
		SniRules: "*.example.com\nhk1.example.com\nhk2.example.com",
	}

	if got := forwardEntryTLSProbeServerName(forward); got != "hk1.example.com" {
		t.Fatalf("expected first concrete SNI host, got %q", got)
	}

	wildcardOnly := &forwardRecord{
		Mode:     forwardModeSNI,
		SniRules: "*.example.com",
	}
	if got := forwardEntryTLSProbeServerName(wildcardOnly); got != "" {
		t.Fatalf("expected wildcard-only rules to skip entry TLS SNI probe, got %q", got)
	}
}

func TestFormatTLSProbeFailure(t *testing.T) {
	if got := formatTLSProbeFailure("Target TCP reachable but", "dial tcp: lookup hk1.example.com: i/o timeout"); got != "Target TCP reachable but TLS probe DNS resolution failed: dial tcp: lookup hk1.example.com: i/o timeout" {
		t.Fatalf("unexpected dns failure message: %q", got)
	}

	if got := formatTLSProbeFailure("Entry TCP reachable but", "remote error: tls: handshake failure"); got != "Entry TCP reachable but TLS handshake failed: remote error: tls: handshake failure" {
		t.Fatalf("unexpected tls failure message: %q", got)
	}
}
