package handler

import (
	"log"
	"net/http"
	"strings"
	"time"

	"go-backend/internal/http/response"
	"go-backend/internal/store/model"
	storeRepo "go-backend/internal/store/repo"
)

const (
	forwardSLAWindow24h        = 24 * time.Hour
	forwardSLAHistoryRetention = 7 * 24 * time.Hour
)

type forwardSLAStatusSummary struct {
	ForwardID           int64   `json:"forwardId"`
	ForwardName         string  `json:"forwardName"`
	UserID              int64   `json:"userId"`
	TunnelID            int64   `json:"tunnelId"`
	Mode                string  `json:"mode"`
	OverallStatus       string  `json:"overallStatus"`
	EntryStatus         string  `json:"entryStatus"`
	TargetStatus        string  `json:"targetStatus"`
	EntryTotal          int     `json:"entryTotal"`
	EntryHealthy        int     `json:"entryHealthy"`
	TargetTotal         int     `json:"targetTotal"`
	TargetHealthy       int     `json:"targetHealthy"`
	CheckedAt           int64   `json:"checkedAt"`
	EntryCheckedAt      int64   `json:"entryCheckedAt"`
	TargetCheckedAt     int64   `json:"targetCheckedAt"`
	Uptime24h           float64 `json:"uptime24h"`
	Samples24h          int     `json:"samples24h"`
	ConsecutiveFailures int     `json:"consecutiveFailures"`
	FirstFailureAt      int64   `json:"firstFailureAt"`
	LastFailureAt       int64   `json:"lastFailureAt"`
	LastHealthyAt       int64   `json:"lastHealthyAt"`
	FailureKind         string  `json:"failureKind"`
	Reason              string  `json:"reason"`
}

func (h *Handler) recordForwardSLAFromCache(forwardID int64) {
	if h == nil || h.repo == nil || forwardID <= 0 {
		return
	}

	h.forwardEntrySummaryMu.RLock()
	entrySummary, entryOK := h.forwardEntrySummaryCache[forwardID]
	entryCheckedAt := h.forwardEntrySummaryChecked[forwardID]
	h.forwardEntrySummaryMu.RUnlock()

	h.forwardTargetSummaryMu.RLock()
	targetSummary, targetOK := h.forwardTargetSummaryCache[forwardID]
	targetCheckedAt := h.forwardTargetSummaryChecked[forwardID]
	h.forwardTargetSummaryMu.RUnlock()

	if !entryOK || !targetOK || entryCheckedAt <= 0 || targetCheckedAt <= 0 {
		return
	}

	forward, err := h.getForwardRecord(forwardID)
	if err != nil || forward == nil {
		return
	}

	state, snapshot := buildForwardSLARecord(forward, entrySummary, entryCheckedAt, targetSummary, targetCheckedAt)
	if err := h.repo.RecordForwardSLA(state, snapshot); err != nil {
		log.Printf("forward_sla: record forward_id=%d err=%v", forwardID, err)
	}
}

func buildForwardSLARecord(forward *forwardRecord, entry forwardEntryStatusSummary, entryCheckedAt int64, target forwardTargetStatusSummary, targetCheckedAt int64) (*model.ForwardSLAState, *model.ForwardSLASnapshot) {
	checkedAt := maxSLAInt64(entryCheckedAt, targetCheckedAt)
	overallStatus := combineForwardSLAStatus(entry.OverallStatus, target.OverallStatus)
	failureKind := classifyForwardSLAFailure(entry, target)
	reason := summarizeForwardSLAReason(entry, target)

	state := &model.ForwardSLAState{
		ForwardID:       forward.ID,
		ForwardName:     forward.Name,
		UserID:          forward.UserID,
		TunnelID:        forward.TunnelID,
		Mode:            normalizeForwardSLAMode(forward.Mode),
		OverallStatus:   overallStatus,
		EntryStatus:     normalizeForwardSLAStatus(entry.OverallStatus),
		TargetStatus:    normalizeForwardSLAStatus(target.OverallStatus),
		EntryTotal:      entry.Total,
		EntryHealthy:    entry.Healthy,
		TargetTotal:     target.Total,
		TargetHealthy:   target.Healthy,
		EntryCheckedAt:  entryCheckedAt,
		TargetCheckedAt: targetCheckedAt,
		CheckedAt:       checkedAt,
		FailureKind:     failureKind,
		Reason:          reason,
	}
	snapshot := &model.ForwardSLASnapshot{
		ForwardID:       state.ForwardID,
		ForwardName:     state.ForwardName,
		UserID:          state.UserID,
		TunnelID:        state.TunnelID,
		Mode:            state.Mode,
		OverallStatus:   state.OverallStatus,
		EntryStatus:     state.EntryStatus,
		TargetStatus:    state.TargetStatus,
		EntryTotal:      state.EntryTotal,
		EntryHealthy:    state.EntryHealthy,
		TargetTotal:     state.TargetTotal,
		TargetHealthy:   state.TargetHealthy,
		FailureKind:     state.FailureKind,
		Reason:          state.Reason,
		EntryCheckedAt:  state.EntryCheckedAt,
		TargetCheckedAt: state.TargetCheckedAt,
		Timestamp:       checkedAt,
	}
	return state, snapshot
}

func normalizeForwardSLAStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "healthy", "partial", "failed":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "unknown"
	}
}

func normalizeForwardSLAMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "sni") {
		return "sni"
	}
	return "direct"
}

func combineForwardSLAStatus(entryStatus, targetStatus string) string {
	entryStatus = normalizeForwardSLAStatus(entryStatus)
	targetStatus = normalizeForwardSLAStatus(targetStatus)

	if entryStatus == "failed" || targetStatus == "failed" {
		return "failed"
	}
	if entryStatus == "partial" || targetStatus == "partial" {
		return "partial"
	}
	if entryStatus == "healthy" && targetStatus == "healthy" {
		return "healthy"
	}
	return "unknown"
}

func summarizeForwardSLAReason(entry forwardEntryStatusSummary, target forwardTargetStatusSummary) string {
	parts := make([]string, 0, 2)
	if normalizeForwardSLAStatus(entry.OverallStatus) == "partial" || normalizeForwardSLAStatus(entry.OverallStatus) == "failed" {
		reason := firstForwardEntryIssueReason(entry)
		if reason == "" {
			reason = "entry health degraded"
		}
		parts = append(parts, "entry: "+reason)
	}
	if normalizeForwardSLAStatus(target.OverallStatus) == "partial" || normalizeForwardSLAStatus(target.OverallStatus) == "failed" {
		reason := firstForwardTargetIssueReason(target)
		if reason == "" {
			reason = "target health degraded"
		}
		parts = append(parts, "target: "+reason)
	}
	return strings.Join(parts, "; ")
}

func firstForwardEntryIssueReason(summary forwardEntryStatusSummary) string {
	for _, item := range summary.Items {
		if item.Healthy {
			continue
		}
		if reason := strings.TrimSpace(item.Reason); reason != "" {
			return reason
		}
		if reason := strings.TrimSpace(item.ApplicationReason); reason != "" {
			return reason
		}
		if item.OccupiedByExternal {
			return "entry port occupied by non-FLVX process"
		}
		if !item.Listening {
			return "entry port is not listening"
		}
		if !item.Reachable {
			return "entry port is not reachable"
		}
	}
	return ""
}

func firstForwardTargetIssueReason(summary forwardTargetStatusSummary) string {
	for _, item := range summary.Items {
		if item.Healthy {
			continue
		}
		if reason := strings.TrimSpace(item.Reason); reason != "" {
			return reason
		}
		if reason := strings.TrimSpace(item.ApplicationReason); reason != "" {
			return reason
		}
	}
	return ""
}

func classifyForwardSLAFailure(entry forwardEntryStatusSummary, target forwardTargetStatusSummary) string {
	entryBad := normalizeForwardSLAStatus(entry.OverallStatus) == "partial" || normalizeForwardSLAStatus(entry.OverallStatus) == "failed"
	targetBad := normalizeForwardSLAStatus(target.OverallStatus) == "partial" || normalizeForwardSLAStatus(target.OverallStatus) == "failed"

	if entryBad && targetBad {
		if targetFailureLooksLikeDNS(target) {
			return "entry+dns"
		}
		return "entry+target"
	}
	if entryBad {
		return "entry"
	}
	if targetBad {
		if targetFailureLooksLikeDNS(target) {
			return "dns"
		}
		if targetFailureLooksLikeApplication(target) {
			return "application"
		}
		return "target"
	}
	return ""
}

func targetFailureLooksLikeDNS(summary forwardTargetStatusSummary) bool {
	for _, item := range summary.Items {
		if item.Healthy {
			continue
		}
		if isTLSProbeResolutionError(item.Reason) || isTLSProbeResolutionError(item.ApplicationReason) {
			return true
		}
	}
	return false
}

func targetFailureLooksLikeApplication(summary forwardTargetStatusSummary) bool {
	for _, item := range summary.Items {
		if item.Healthy {
			continue
		}
		if item.ApplicationChecked && !item.ApplicationHealthy {
			return true
		}
		if strings.Contains(strings.ToLower(item.Reason), "tls") {
			return true
		}
	}
	return false
}

func maxSLAInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (h *Handler) forwardSLAStatusBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := decodeJSON(r.Body, &req); err != nil || len(req.IDs) == 0 {
		response.WriteJSON(w, response.ErrDefault("invalid request"))
		return
	}

	actorUserID, actorRole, err := userRoleFromRequest(r)
	if err != nil {
		response.WriteJSON(w, response.Err(401, "invalid token"))
		return
	}

	forwardsByID := make(map[int64]*forwardRecord)
	accessibleIDs := make([]int64, 0, len(req.IDs))
	seen := make(map[int64]struct{}, len(req.IDs))
	for _, id := range req.IDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		forward, accessErr := h.ensureForwardAccessByActor(actorUserID, actorRole, id)
		if accessErr != nil || forward == nil {
			continue
		}
		forwardsByID[id] = forward
		accessibleIDs = append(accessibleIDs, id)
		h.recordForwardSLAFromCache(id)
		h.enqueueForwardEntrySummaryRefresh(id)
		h.enqueueForwardTargetSummaryRefresh(id)
	}

	if len(accessibleIDs) == 0 {
		response.WriteJSON(w, response.OK([]forwardSLAStatusSummary{}))
		return
	}

	now := time.Now().UnixMilli()
	states, err := h.repo.GetForwardSLAStates(accessibleIDs)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	windowSummaries, err := h.repo.GetForwardSLAWindowSummaries(accessibleIDs, now-int64(forwardSLAWindow24h/time.Millisecond), now)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}

	items := make([]forwardSLAStatusSummary, 0, len(accessibleIDs))
	for _, id := range accessibleIDs {
		forward := forwardsByID[id]
		state, ok := states[id]
		if !ok {
			items = append(items, buildUnknownForwardSLASummary(forward, windowSummaries[id]))
			continue
		}
		summary := buildForwardSLASummaryFromState(state, windowSummaries[id])
		items = append(items, summary)
	}

	response.WriteJSON(w, response.OK(items))
}

func buildUnknownForwardSLASummary(forward *forwardRecord, windowSummary storeRepo.ForwardSLAWindowSummary) forwardSLAStatusSummary {
	item := forwardSLAStatusSummary{
		OverallStatus: "unknown",
		EntryStatus:   "unknown",
		TargetStatus:  "unknown",
		Uptime24h:     windowSummary.Uptime,
		Samples24h:    windowSummary.Samples,
	}
	if forward != nil {
		item.ForwardID = forward.ID
		item.ForwardName = forward.Name
		item.UserID = forward.UserID
		item.TunnelID = forward.TunnelID
		item.Mode = normalizeForwardSLAMode(forward.Mode)
	}
	return item
}

func buildForwardSLASummaryFromState(state model.ForwardSLAState, windowSummary storeRepo.ForwardSLAWindowSummary) forwardSLAStatusSummary {
	uptime24h := state.Uptime24h
	samples24h := state.Samples24h
	if windowSummary.Samples > 0 {
		uptime24h = windowSummary.Uptime
		samples24h = windowSummary.Samples
	}

	item := forwardSLAStatusSummary{
		ForwardID:           state.ForwardID,
		ForwardName:         state.ForwardName,
		UserID:              state.UserID,
		TunnelID:            state.TunnelID,
		Mode:                state.Mode,
		OverallStatus:       normalizeForwardSLAStatus(state.OverallStatus),
		EntryStatus:         normalizeForwardSLAStatus(state.EntryStatus),
		TargetStatus:        normalizeForwardSLAStatus(state.TargetStatus),
		EntryTotal:          state.EntryTotal,
		EntryHealthy:        state.EntryHealthy,
		TargetTotal:         state.TargetTotal,
		TargetHealthy:       state.TargetHealthy,
		CheckedAt:           state.CheckedAt,
		EntryCheckedAt:      state.EntryCheckedAt,
		TargetCheckedAt:     state.TargetCheckedAt,
		Uptime24h:           uptime24h,
		Samples24h:          samples24h,
		ConsecutiveFailures: state.ConsecutiveFailures,
		FirstFailureAt:      state.FirstFailureAt,
		LastFailureAt:       state.LastFailureAt,
		LastHealthyAt:       state.LastHealthyAt,
		FailureKind:         state.FailureKind,
		Reason:              state.Reason,
	}
	return item
}
