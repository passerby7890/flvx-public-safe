package handler

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	nodeNetworkSyncInterval = 5 * time.Minute
	nodeNetworkSyncTimeout  = 20 * time.Second
	nodeNetworkSyncWorkers  = 4
)

type nodeNetworkInfoSnapshot struct {
	IPv4 []string
	IPv6 []string
}

func (h *Handler) runNodeNetworkSyncLoop(ctx context.Context) {
	defer h.jobsWG.Done()

	ticker := time.NewTicker(nodeNetworkSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.syncAllOnlineNodeNetworkInfo()
		}
	}
}

func (h *Handler) syncAllOnlineNodeNetworkInfo() {
	if h == nil || h.repo == nil {
		return
	}

	nodes, err := h.repo.ListOnlineLocalNodeRecords()
	if err != nil || len(nodes) == 0 {
		return
	}

	sem := make(chan struct{}, nodeNetworkSyncWorkers)
	var wg sync.WaitGroup

	for i := range nodes {
		nodeID := nodes[i].ID
		if nodeID <= 0 {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := h.syncNodeNetworkInfo(nodeID); err != nil {
				fmt.Printf("node network sync: node %d failed: %v\n", nodeID, err)
			}
		}()
	}

	wg.Wait()
}

func (h *Handler) syncNodeNetworkInfo(nodeID int64) error {
	if h == nil || h.repo == nil || h.wsServer == nil || nodeID <= 0 {
		return nil
	}

	node, err := h.repo.GetNodeRecord(nodeID)
	if err != nil || node == nil {
		return err
	}
	if node.IsRemote == 1 {
		return nil
	}

	observedIP := strings.TrimSpace(h.wsServer.GetObservedNodeIP(nodeID))
	info, infoErr := h.fetchNodeNetworkInfo(nodeID)
	if infoErr != nil && !isGetNetworkInfoUnsupportedError(infoErr) && observedIP == "" {
		return infoErr
	}

	serverIP, serverIPv4, serverIPv6, extraIPs := reconcileNodeNetworkFields(node, info, observedIP)
	if strings.TrimSpace(serverIP) == "" {
		return nil
	}

	if strings.TrimSpace(node.ServerIP) == serverIP &&
		strings.TrimSpace(node.ServerIPv4) == serverIPv4 &&
		strings.TrimSpace(node.ServerIPv6) == serverIPv6 &&
		normalizeExtraIPList(node.ExtraIPs) == normalizeExtraIPList(extraIPs) {
		return nil
	}

	return h.repo.UpdateNodeNetworkInfo(nodeID, serverIP, serverIPv4, serverIPv6, extraIPs, time.Now().UnixMilli())
}

func (h *Handler) fetchNodeNetworkInfo(nodeID int64) (*nodeNetworkInfoSnapshot, error) {
	if h == nil || h.wsServer == nil {
		return nil, nil
	}

	result, err := h.wsServer.SendCommand(nodeID, "GetNetworkInfo", map[string]interface{}{}, nodeNetworkSyncTimeout)
	if err != nil {
		return nil, err
	}

	return &nodeNetworkInfoSnapshot{
		IPv4: stringSliceFromAny(result.Data["ipv4"]),
		IPv6: stringSliceFromAny(result.Data["ipv6"]),
	}, nil
}

func isGetNetworkInfoUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "getnetworkinfo") &&
		(strings.Contains(message, "unknown command") || strings.Contains(message, "未知命令"))
}

func stringSliceFromAny(value interface{}) []string {
	switch items := value.(type) {
	case []string:
		return append([]string(nil), items...)
	case []interface{}:
		result := make([]string, 0, len(items))
		for _, item := range items {
			text := strings.TrimSpace(asString(item))
			if text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func reconcileNodeNetworkFields(existing *nodeRecord, info *nodeNetworkInfoSnapshot, observedIP string) (string, string, string, string) {
	existingServerIP := strings.TrimSpace(existing.ServerIP)
	existingIPv4 := strings.TrimSpace(existing.ServerIPv4)
	existingIPv6 := strings.TrimSpace(existing.ServerIPv6)

	observedAddr, hasObserved := normalizeAddrLiteral(observedIP)

	reportedIPv4 := infoIPv4(info)
	reportedIPv6 := infoIPv6(info)

	// Prefer addresses from the node's own interfaces. The websocket source can
	// be a WARP/control-plane route that is not the public inbound entry IP.
	ipv4Candidates := make([]string, 0, len(reportedIPv4)+1)
	ipv6Candidates := make([]string, 0, len(reportedIPv6)+1)
	ipv4Candidates = append(ipv4Candidates, reportedIPv4...)
	ipv6Candidates = append(ipv6Candidates, reportedIPv6...)
	if hasObserved {
		if observedAddr.Is4() {
			ipv4Candidates = append(ipv4Candidates, observedAddr.String())
		} else if observedAddr.Is6() {
			ipv6Candidates = append(ipv6Candidates, observedAddr.String())
		}
	}

	ipv4List := normalizeNodeIPList(ipv4Candidates...)
	ipv6List := normalizeNodeIPList(ipv6Candidates...)
	reportedIPv4List := normalizeNodeIPList(reportedIPv4...)
	reportedIPv6List := normalizeNodeIPList(reportedIPv6...)

	serverIPv4 := firstNonEmpty(preferredNodeAddress(reportedIPv4List, existingIPv4), firstItem(ipv4List), existingIPv4)
	serverIPv6 := firstNonEmpty(preferredNodeAddress(reportedIPv6List, existingIPv6), firstItem(ipv6List), existingIPv6)
	serverIP := firstNonEmpty(serverIPv4, serverIPv6, observedIP, existingServerIP)

	extraIPs := strings.TrimSpace(existing.ExtraIPs)
	reportedAny := info != nil && (len(info.IPv4) > 0 || len(info.IPv6) > 0)
	if reportedAny {
		remaining := make([]string, 0, len(ipv4List)+len(ipv6List))
		for _, value := range append(append([]string{}, ipv4List...), ipv6List...) {
			if value == "" || value == serverIP || value == serverIPv4 || value == serverIPv6 {
				continue
			}
			remaining = append(remaining, value)
		}
		extraIPs = strings.Join(normalizeNodeIPList(remaining...), ",")
	}

	return strings.TrimSpace(serverIP), strings.TrimSpace(serverIPv4), strings.TrimSpace(serverIPv6), strings.TrimSpace(extraIPs)
}

func infoIPv4(info *nodeNetworkInfoSnapshot) []string {
	if info == nil {
		return nil
	}
	return info.IPv4
}

func infoIPv6(info *nodeNetworkInfoSnapshot) []string {
	if info == nil {
		return nil
	}
	return info.IPv6
}

func normalizeNodeIPList(values ...string) []string {
	publicItems := make([]string, 0, len(values))
	privateItems := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, raw := range values {
		addr, ok := normalizeAddrLiteral(raw)
		if !ok {
			continue
		}
		text := addr.String()
		if _, exists := seen[text]; exists {
			continue
		}
		seen[text] = struct{}{}
		if isPublicRoutableAddr(addr) {
			publicItems = append(publicItems, text)
		} else {
			privateItems = append(privateItems, text)
		}
	}

	return append(publicItems, privateItems...)
}

func normalizeAddrLiteral(raw string) (netip.Addr, bool) {
	value := strings.Trim(strings.TrimSpace(raw), "\"[]")
	if value == "" {
		return netip.Addr{}, false
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsLoopback() || addr.IsMulticast() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() {
		return netip.Addr{}, false
	}
	return addr, true
}

func isPublicRoutableAddr(addr netip.Addr) bool {
	return addr.IsGlobalUnicast() &&
		!addr.IsPrivate() &&
		!addr.IsLoopback() &&
		!addr.IsLinkLocalUnicast() &&
		!addr.IsMulticast() &&
		!addr.IsUnspecified()
}

func firstItem(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func preferredNodeAddress(values []string, existing string) string {
	existingAddr, ok := normalizeAddrLiteral(existing)
	if ok {
		existingText := existingAddr.String()
		for _, value := range values {
			if strings.TrimSpace(value) == existingText {
				return existingText
			}
		}
	}
	return firstItem(values)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeExtraIPList(raw string) string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	return strings.Join(normalizeNodeIPList(parts...), ",")
}
