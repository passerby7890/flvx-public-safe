package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	batchJobStatusPending   = "pending"
	batchJobStatusRunning   = "running"
	batchJobStatusCompleted = "completed"
	batchJobStatusFailed    = "failed"
	batchJobRetention       = 24 * time.Hour
)

type batchJobSnapshot struct {
	JobID        string               `json:"jobId"`
	Action       string               `json:"action"`
	Status       string               `json:"status"`
	Total        int                  `json:"total"`
	Completed    int                  `json:"completed"`
	SuccessCount int                  `json:"successCount"`
	FailCount    int                  `json:"failCount"`
	Failures     []batchFailureDetail `json:"failures,omitempty"`
	Message      string               `json:"message,omitempty"`
	CreatedAt    int64                `json:"createdAt"`
	UpdatedAt    int64                `json:"updatedAt"`
	FinishedAt   int64                `json:"finishedAt,omitempty"`
	OwnerUserID  int64                `json:"-"`
	OwnerRoleID  int                  `json:"-"`
}

type batchJobManager struct {
	mu   sync.RWMutex
	jobs map[string]*batchJobSnapshot
}

func newBatchJobManager() *batchJobManager {
	return &batchJobManager{
		jobs: make(map[string]*batchJobSnapshot),
	}
}

func newBatchJobID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	}
	return hex.EncodeToString(buf)
}

func (m *batchJobManager) pruneLocked(now time.Time) {
	if m == nil {
		return
	}
	cutoff := now.Add(-batchJobRetention).UnixMilli()
	for id, job := range m.jobs {
		if job == nil {
			delete(m.jobs, id)
			continue
		}
		updatedAt := job.UpdatedAt
		if updatedAt == 0 {
			updatedAt = job.CreatedAt
		}
		if updatedAt > 0 && updatedAt < cutoff {
			delete(m.jobs, id)
		}
	}
}

func (m *batchJobManager) create(action string, ownerUserID int64, ownerRoleID int, total int) *batchJobSnapshot {
	if m == nil {
		return nil
	}
	now := time.Now().UnixMilli()
	job := &batchJobSnapshot{
		JobID:       newBatchJobID(),
		Action:      strings.TrimSpace(action),
		Status:      batchJobStatusPending,
		Total:       total,
		CreatedAt:   now,
		UpdatedAt:   now,
		OwnerUserID: ownerUserID,
		OwnerRoleID: ownerRoleID,
		Failures:    make([]batchFailureDetail, 0),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(time.Now())
	m.jobs[job.JobID] = job
	return cloneBatchJob(job)
}

func (m *batchJobManager) get(jobID string) (*batchJobSnapshot, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	job, ok := m.jobs[strings.TrimSpace(jobID)]
	m.mu.RUnlock()
	if !ok || job == nil {
		return nil, false
	}
	return cloneBatchJob(job), true
}

func (m *batchJobManager) update(jobID string, fn func(job *batchJobSnapshot)) (*batchJobSnapshot, error) {
	if m == nil {
		return nil, errors.New("batch job manager not initialized")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[strings.TrimSpace(jobID)]
	if !ok || job == nil {
		return nil, errors.New("batch job not found")
	}
	fn(job)
	job.UpdatedAt = time.Now().UnixMilli()
	return cloneBatchJob(job), nil
}

func (m *batchJobManager) markRunning(jobID string) {
	_, _ = m.update(jobID, func(job *batchJobSnapshot) {
		job.Status = batchJobStatusRunning
		job.Message = ""
	})
}

func (m *batchJobManager) applyResult(jobID string, result forwardBatchTaskResult) {
	_, _ = m.update(jobID, func(job *batchJobSnapshot) {
		job.Completed++
		if result.Success {
			job.SuccessCount++
			return
		}
		job.FailCount++
		reason := normalizeBatchFailureReason(errString(result.Err))
		if reason == "" {
			reason = "未知错误"
		}
		job.Failures = append(job.Failures, batchFailureDetail{
			ID:     result.ID,
			Name:   strings.TrimSpace(result.Name),
			Reason: reason,
		})
	})
}

func (m *batchJobManager) complete(jobID string) {
	_, _ = m.update(jobID, func(job *batchJobSnapshot) {
		job.Status = batchJobStatusCompleted
		job.Message = "completed"
		job.FinishedAt = time.Now().UnixMilli()
	})
}

func (m *batchJobManager) fail(jobID string, message string) {
	_, _ = m.update(jobID, func(job *batchJobSnapshot) {
		job.Status = batchJobStatusFailed
		job.Message = strings.TrimSpace(message)
		job.FinishedAt = time.Now().UnixMilli()
	})
}

func cloneBatchJob(job *batchJobSnapshot) *batchJobSnapshot {
	if job == nil {
		return nil
	}
	copyJob := *job
	if job.Failures != nil {
		copyJob.Failures = append([]batchFailureDetail(nil), job.Failures...)
	}
	return &copyJob
}
