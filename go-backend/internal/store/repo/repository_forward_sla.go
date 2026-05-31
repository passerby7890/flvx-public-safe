package repo

import (
	"errors"

	"go-backend/internal/store/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ForwardSLASnapshotRetentionMs int64 = 7 * 24 * 60 * 60 * 1000

type ForwardSLAWindowSummary struct {
	ForwardID       int64
	Samples         int
	ScoredSamples   int
	HealthySamples  int
	PartialSamples  int
	FailedSamples   int
	UnknownSamples  int
	AvailabilitySum float64
	Uptime          float64
}

func (r *Repository) RecordForwardSLA(state *model.ForwardSLAState, snapshot *model.ForwardSLASnapshot) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	if state == nil || snapshot == nil || state.ForwardID <= 0 {
		return nil
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		var previous model.ForwardSLAState
		err := tx.Where("forward_id = ?", state.ForwardID).First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := state.CheckedAt
		if now <= 0 {
			now = snapshot.Timestamp
		}
		if now <= 0 {
			return nil
		}

		if previous.ForwardID > 0 {
			state.CreatedTime = previous.CreatedTime
			state.LastFailureAt = previous.LastFailureAt
			state.LastHealthyAt = previous.LastHealthyAt
			state.FirstFailureAt = previous.FirstFailureAt
			state.ConsecutiveFailures = previous.ConsecutiveFailures
		} else {
			state.CreatedTime = now
		}
		state.UpdatedTime = now

		switch state.OverallStatus {
		case "healthy":
			state.ConsecutiveFailures = 0
			state.FirstFailureAt = 0
			state.LastHealthyAt = now
		case "partial", "failed":
			state.ConsecutiveFailures = previous.ConsecutiveFailures + 1
			if previous.FirstFailureAt > 0 && previous.OverallStatus != "healthy" {
				state.FirstFailureAt = previous.FirstFailureAt
			} else {
				state.FirstFailureAt = now
			}
			state.LastFailureAt = now
		default:
			// Unknown means the probe did not have enough data. Keep streaks as-is
			// so a transient cache miss does not create a false SLA incident.
		}

		duplicateSnapshot := previous.ForwardID > 0 &&
			previous.EntryCheckedAt == state.EntryCheckedAt &&
			previous.TargetCheckedAt == state.TargetCheckedAt

		if !duplicateSnapshot {
			if err := tx.Create(snapshot).Error; err != nil {
				return err
			}
		}

		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "forward_id"}},
			UpdateAll: true,
		}).Create(state).Error
	})
}

func (r *Repository) GetForwardSLAStates(ids []int64) (map[int64]model.ForwardSLAState, error) {
	result := make(map[int64]model.ForwardSLAState)
	normalizedIDs := normalizeInt64IDs(ids)
	if r == nil || r.db == nil || len(normalizedIDs) == 0 {
		return result, nil
	}

	var rows []model.ForwardSLAState
	if err := r.db.Where("forward_id IN ?", normalizedIDs).Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		if row.ForwardID <= 0 {
			continue
		}
		result[row.ForwardID] = row
	}
	return result, nil
}

func (r *Repository) DeleteForwardSLAStates(ids []int64) error {
	if r == nil || r.db == nil {
		return errors.New("repository not initialized")
	}
	normalizedIDs := normalizeInt64IDs(ids)
	if len(normalizedIDs) == 0 {
		return nil
	}
	return r.db.Where("forward_id IN ?", normalizedIDs).Delete(&model.ForwardSLAState{}).Error
}

func (r *Repository) GetForwardSLAWindowSummaries(ids []int64, startMs, endMs int64) (map[int64]ForwardSLAWindowSummary, error) {
	result := make(map[int64]ForwardSLAWindowSummary)
	normalizedIDs := normalizeInt64IDs(ids)
	if r == nil || r.db == nil || len(normalizedIDs) == 0 {
		return result, nil
	}

	var rows []model.ForwardSLASnapshot
	err := r.db.
		Where("forward_id IN ? AND timestamp >= ? AND timestamp <= ?", normalizedIDs, startMs, endMs).
		Order("timestamp ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		if row.ForwardID <= 0 {
			continue
		}
		summary := result[row.ForwardID]
		summary.ForwardID = row.ForwardID
		summary.Samples++
		switch row.OverallStatus {
		case "healthy":
			summary.HealthySamples++
		case "partial":
			summary.PartialSamples++
		case "failed":
			summary.FailedSamples++
		default:
			summary.UnknownSamples++
		}
		if score, ok := forwardSLASnapshotAvailability(row); ok {
			summary.AvailabilitySum += score
			summary.ScoredSamples++
		}
		result[row.ForwardID] = summary
	}

	for id, summary := range result {
		if summary.ScoredSamples > 0 {
			summary.Uptime = summary.AvailabilitySum / float64(summary.ScoredSamples)
		}
		result[id] = summary
	}

	return result, nil
}

func forwardSLASnapshotAvailability(row model.ForwardSLASnapshot) (float64, bool) {
	overall := normalizeForwardSLARepoStatus(row.OverallStatus)
	if overall == "unknown" {
		return 0, false
	}

	entryScore, entryOK := forwardSLAComponentAvailability(row.EntryStatus, row.EntryHealthy, row.EntryTotal)
	targetScore, targetOK := forwardSLAComponentAvailability(row.TargetStatus, row.TargetHealthy, row.TargetTotal)
	if !entryOK || !targetOK {
		return 0, false
	}
	if entryScore < targetScore {
		return entryScore, true
	}
	return targetScore, true
}

func forwardSLAComponentAvailability(status string, healthy, total int) (float64, bool) {
	switch normalizeForwardSLARepoStatus(status) {
	case "unknown":
		return 0, false
	case "healthy":
		if total <= 0 {
			return 1, true
		}
	case "partial", "failed":
		if total <= 0 {
			return 0, true
		}
	default:
		return 0, false
	}
	if healthy < 0 {
		healthy = 0
	}
	if healthy > total {
		healthy = total
	}
	return float64(healthy) / float64(total), true
}

func normalizeForwardSLARepoStatus(status string) string {
	switch status {
	case "healthy", "partial", "failed":
		return status
	default:
		return "unknown"
	}
}

func (r *Repository) PruneForwardSLASnapshots(olderThanMs int64) error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Where("timestamp < ?", olderThanMs).Delete(&model.ForwardSLASnapshot{}).Error
}

func normalizeInt64IDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
