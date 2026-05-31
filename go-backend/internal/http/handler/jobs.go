package handler

import (
	"context"
	"fmt"
	"time"
)

func (h *Handler) StartBackgroundJobs() {
	if h == nil || h.repo == nil {
		return
	}

	h.jobsMu.Lock()
	if h.jobsStarted {
		h.jobsMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.jobsCancel = cancel
	h.jobsStarted = true
	h.jobsWG.Add(8)
	h.jobsMu.Unlock()

	go h.runHourlyStatsLoop(ctx)
	go h.runDailyMaintenanceLoop(ctx)
	go h.runNodeRenewalCycleLoop(ctx)
	go h.runMetricsIngestion(ctx)
	go h.runHealthChecks(ctx)
	go h.runTunnelQualityProber(ctx)
	go h.runForwardRuntimeReconcileLoop(ctx)
	go h.runNodeNetworkSyncLoop(ctx)
}

func (h *Handler) StopBackgroundJobs() {
	if h == nil {
		return
	}

	h.jobsMu.Lock()
	if !h.jobsStarted {
		h.jobsMu.Unlock()
		return
	}
	cancel := h.jobsCancel
	h.jobsCancel = nil
	h.jobsStarted = false
	h.jobsMu.Unlock()

	if cancel != nil {
		cancel()
	}
	h.jobsWG.Wait()
}

func (h *Handler) runMetricsIngestion(ctx context.Context) {
	defer h.jobsWG.Done()
	if h.metrics != nil {
		h.metrics.Start(ctx)
	}
}

func (h *Handler) runHealthChecks(ctx context.Context) {
	defer h.jobsWG.Done()
	if h.healthCheck != nil {
		h.healthCheck.Start(ctx)
	}
}

func (h *Handler) runTunnelQualityProber(ctx context.Context) {
	defer h.jobsWG.Done()
	if h == nil || h.qualityProber == nil || !h.isTunnelQualityMonitoringEnabled() {
		return
	}

	h.qualityProber.Start(ctx)
}

func (h *Handler) runForwardRuntimeReconcileLoop(ctx context.Context) {
	defer h.jobsWG.Done()

	h.runForwardRuntimeReconcile()

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.runForwardRuntimeReconcile()
		}
	}
}

func (h *Handler) runForwardRuntimeReconcile() {
	if h == nil || h.repo == nil {
		return
	}

	h.runTunnelForwardEntryConsistency()

	forwards, err := h.repo.ListActiveForwards()
	if err != nil || len(forwards) == 0 {
		return
	}

	for i := range forwards {
		forward := &forwards[i]
		if forward == nil {
			continue
		}
		statuses, inspectErr := h.inspectForwardEntrypoints(forward)
		if inspectErr != nil || len(statuses) == 0 {
			continue
		}
		needsRepair := false
		for _, item := range statuses {
			if !shouldAutoRepairForwardEntryStatus(item) {
				continue
			}
			needsRepair = true
			break
		}
		if !needsRepair {
			continue
		}
		_ = h.syncForwardServices(forward, "UpdateService", true)
	}
}

func (h *Handler) runTunnelForwardEntryConsistency() {
	if h == nil || h.repo == nil {
		return
	}

	tunnelIDs, err := h.repo.ListEnabledTunnelIDs()
	if err != nil || len(tunnelIDs) == 0 {
		return
	}

	for _, tunnelID := range tunnelIDs {
		if tunnelID <= 0 {
			continue
		}
		if err := h.reconcileTunnelForwardEntryConsistency(tunnelID); err != nil {
			fmt.Printf("tunnel forward entry consistency: tunnel %d failed: %v\n", tunnelID, err)
		}
	}
}

func (h *Handler) reconcileTunnelForwardEntryConsistency(tunnelID int64) error {
	if h == nil || h.repo == nil || tunnelID <= 0 {
		return nil
	}

	entryNodeIDs, err := h.tunnelEntryNodeIDs(tunnelID)
	if err != nil || len(entryNodeIDs) == 0 {
		return err
	}

	forwards, err := h.repo.ListActiveForwardsByTunnel(tunnelID)
	if err != nil || len(forwards) == 0 {
		return err
	}

	needsRepair := false
	for i := range forwards {
		forward := &forwards[i]
		if forward == nil || forward.ID <= 0 {
			continue
		}

		ports, portErr := h.listForwardPorts(forward.ID)
		if portErr != nil {
			return portErr
		}
		if len(ports) == 0 || !sameInt64Set(entryNodeIDs, forwardPortNodeIDs(ports)) {
			needsRepair = true
			break
		}
	}

	if !needsRepair {
		return nil
	}

	h.syncTunnelForwardsEntryPorts(tunnelID, entryNodeIDs)
	for i := range forwards {
		if forwards[i].ID <= 0 {
			continue
		}
		h.invalidateForwardEntrySummary(forwards[i].ID)
		h.enqueueForwardEntrySummaryRefresh(forwards[i].ID)
	}
	if h.wsServer == nil {
		return nil
	}
	return h.redeployTunnelAndForwards(tunnelID)
}

func (h *Handler) runHourlyStatsLoop(ctx context.Context) {
	defer h.jobsWG.Done()

	for {
		wait := durationUntilNextHour(time.Now())
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			h.runStatisticsFlowJob(time.Now())
		}
	}
}

func (h *Handler) runDailyMaintenanceLoop(ctx context.Context) {
	defer h.jobsWG.Done()

	for {
		wait := durationUntilNextDailyMaintenance(time.Now())
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			h.runResetAndExpiryJob(time.Now())
		}
	}
}

func durationUntilNextHour(now time.Time) time.Duration {
	next := now.Truncate(time.Hour).Add(time.Hour)
	return next.Sub(now)
}

func durationUntilNextDailyMaintenance(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 5, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

func (h *Handler) runStatisticsFlowJob(now time.Time) {
	if h == nil || h.repo == nil {
		return
	}

	nowMs := now.UnixMilli()
	cutoffMs := nowMs - int64((48*time.Hour)/time.Millisecond)
	_ = h.repo.PurgeOldStatisticsFlows(cutoffMs)

	hourMark := now.Truncate(time.Hour)
	hourText := hourMark.Format("15:04")
	createdTime := hourMark.UnixMilli()

	users, err := h.repo.ListAllUserFlowSnapshots()
	if err != nil {
		return
	}

	for _, user := range users {
		currentTotal := user.InFlow + user.OutFlow
		increment := currentTotal

		lastTotal, err := h.repo.GetLastStatisticsFlowTotal(user.UserID)
		if err == nil && lastTotal.Valid {
			increment = currentTotal - lastTotal.Int64
			if increment < 0 {
				increment = currentTotal
			}
		}

		_ = h.repo.CreateStatisticsFlow(user.UserID, increment, currentTotal, hourText, createdTime)
	}
}

func (h *Handler) runResetAndExpiryJob(now time.Time) {
	if h == nil || h.repo == nil {
		return
	}

	h.resetMonthlyFlow(now)
	h.resetUserQuotaWindows(now)
	h.disableExpiredUsers(now.UnixMilli())
	h.disableExpiredUserTunnels(now.UnixMilli())
	h.pruneForwardSLAHistory(now)
}

func (h *Handler) pruneForwardSLAHistory(now time.Time) {
	if h == nil || h.repo == nil {
		return
	}
	cutoffMs := now.Add(-forwardSLAHistoryRetention).UnixMilli()
	_ = h.repo.PruneForwardSLASnapshots(cutoffMs)
}

func (h *Handler) resetMonthlyFlow(now time.Time) {
	currentDay := now.Day()
	lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()

	_ = h.repo.ResetUserMonthlyFlow(currentDay, lastDay)
	_ = h.repo.ResetUserTunnelMonthlyFlow(currentDay, lastDay)
}

func (h *Handler) disableExpiredUsers(nowMs int64) {
	userIDs, err := h.repo.ListExpiredActiveUserIDs(nowMs)
	if err != nil {
		return
	}

	for _, userID := range userIDs {
		forwards, err := h.listActiveForwardsByUser(userID)
		if err == nil {
			h.pauseForwardRecords(forwards, nowMs)
		}
		_ = h.repo.DisableUser(userID)
	}
}

func (h *Handler) disableExpiredUserTunnels(nowMs int64) {
	items, err := h.repo.ListExpiredActiveUserTunnels(nowMs)
	if err != nil {
		return
	}

	for _, item := range items {
		forwards, err := h.listActiveForwardsByUserTunnel(item.UserID, item.TunnelID)
		if err == nil {
			h.pauseForwardRecords(forwards, nowMs)
		}
		_ = h.repo.DisableUserTunnel(item.ID)
	}
}

func (h *Handler) runNodeRenewalCycleLoop(ctx context.Context) {
	defer h.jobsWG.Done()

	for {
		wait := durationUntilNextNodeRenewalCycle(time.Now())
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			h.runNodeRenewalCycleJob(time.Now())
		}
	}
}

func durationUntilNextNodeRenewalCycle(now time.Time) time.Duration {
	next := now.Truncate(6 * time.Hour).Add(6 * time.Hour)
	return next.Sub(now)
}

func (h *Handler) runNodeRenewalCycleJob(now time.Time) {
	if h == nil || h.repo == nil {
		return
	}

	advanced, err := h.repo.AdvanceNodeRenewalCycles(now.UnixMilli())
	if err != nil {
		return
	}

	_ = advanced
}
