package store

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Review job lifecycle for the durable PR review queue.
const (
	ReviewJobPending    = "pending"
	ReviewJobProcessing = "processing"
	ReviewJobCompleted  = "completed"
	ReviewJobFailed     = "failed"
)

// ReviewJob is one queued GitHub PR review (persisted across restarts).
type ReviewJob struct {
	ID         uint `gorm:"primaryKey"`
	FullName   string `gorm:"not null"`
	PRNumber   int    `gorm:"not null"`
	Status     string `gorm:"not null;index"`
	Error      string `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"index"`
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// EnqueueReviewJob adds a pending job. Any existing pending job for the same
// repo and PR is replaced so rapid pushes collapse to one review.
func (s *Store) EnqueueReviewJob(fullName string, prNumber int) (uint, error) {
	fn, err := normalizeFullName(fullName)
	if err != nil {
		return 0, err
	}
	if prNumber <= 0 {
		return 0, fmt.Errorf("invalid pr number")
	}

	job := ReviewJob{
		FullName:  fn,
		PRNumber:  prNumber,
		Status:    ReviewJobPending,
		CreatedAt: time.Now().UTC(),
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("full_name = ? AND pr_number = ? AND status = ?", fn, prNumber, ReviewJobPending).
			Delete(&ReviewJob{}).Error; err != nil {
			return err
		}
		return tx.Create(&job).Error
	})
	if err != nil {
		return 0, err
	}
	return job.ID, nil
}

// ResetStaleProcessingJobs marks interrupted processing jobs as pending again
// (e.g. after a crash). Call once when starting the queue worker.
func (s *Store) ResetStaleProcessingJobs() error {
	return s.db.Model(&ReviewJob{}).
		Where("status = ?", ReviewJobProcessing).
		Updates(map[string]interface{}{
			"status":     ReviewJobPending,
			"started_at": nil,
		}).Error
}

// ClaimNextReviewJob returns the oldest pending job after marking it processing, or nil if none.
// Safe with a single queue worker (no cross-process concurrency).
func (s *Store) ClaimNextReviewJob() (*ReviewJob, error) {
	var job ReviewJob
	err := s.db.Where("status = ?", ReviewJobPending).Order("created_at ASC").First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	res := s.db.Model(&ReviewJob{}).
		Where("id = ? AND status = ?", job.ID, ReviewJobPending).
		Updates(map[string]interface{}{
			"status":     ReviewJobProcessing,
			"started_at": now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	job.Status = ReviewJobProcessing
	job.StartedAt = &now
	return &job, nil
}

// FinishReviewJob marks a job completed or failed.
func (s *Store) FinishReviewJob(id uint, jobErr error) error {
	now := time.Now().UTC()
	status := ReviewJobCompleted
	errStr := ""
	if jobErr != nil {
		status = ReviewJobFailed
		errStr = jobErr.Error()
	}
	return s.db.Model(&ReviewJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      status,
		"error":       errStr,
		"finished_at": now,
	}).Error
}

// ReviewQueueCounts is the number of jobs per status.
type ReviewQueueCounts struct {
	Pending    int64
	Processing int64
	Completed  int64
	Failed     int64
}

// ReviewQueueStatus returns aggregate queue counts and the current processing job if any.
func (s *Store) ReviewQueueStatus() (ReviewQueueCounts, *ReviewJob, error) {
	var rows []struct {
		Status string
		N      int64
	}
	if err := s.db.Model(&ReviewJob{}).
		Select("status, COUNT(*) AS n").
		Group("status").
		Scan(&rows).Error; err != nil {
		return ReviewQueueCounts{}, nil, err
	}
	var c ReviewQueueCounts
	for _, r := range rows {
		switch r.Status {
		case ReviewJobPending:
			c.Pending = r.N
		case ReviewJobProcessing:
			c.Processing = r.N
		case ReviewJobCompleted:
			c.Completed = r.N
		case ReviewJobFailed:
			c.Failed = r.N
		}
	}
	var proc *ReviewJob
	if c.Processing > 0 {
		var j ReviewJob
		if err := s.db.Where("status = ?", ReviewJobProcessing).Order("started_at ASC").First(&j).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return c, nil, err
			}
		} else {
			proc = &j
		}
	}
	return c, proc, nil
}

func (s *Store) reviewJobsScope(statusFilter string) *gorm.DB {
	q := s.db.Model(&ReviewJob{})
	if statusFilter != "" {
		q = q.Where("status = ?", statusFilter)
	}
	return q
}

// ListReviewJobs returns jobs newest first. statusFilter empty means all statuses.
func (s *Store) ListReviewJobs(limit, offset int, statusFilter string) ([]ReviewJob, int64, error) {
	var total int64
	if err := s.reviewJobsScope(statusFilter).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var jobs []ReviewJob
	err := s.reviewJobsScope(statusFilter).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&jobs).Error
	if err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

// GetReviewJob loads a job by primary key.
func (s *Store) GetReviewJob(id uint) (*ReviewJob, error) {
	var j ReviewJob
	if err := s.db.First(&j, id).Error; err != nil {
		return nil, err
	}
	return &j, nil
}
