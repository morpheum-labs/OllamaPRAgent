package server

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/morpheum-labs/OllamaPRAgent/internal/store"
)

type jobPublic struct {
	ID         uint    `json:"id"`
	FullName   string  `json:"full_name"`
	PRNumber   int     `json:"pr_number"`
	Status     string  `json:"status"`
	Error      string  `json:"error,omitempty"`
	CreatedAt  string  `json:"created_at"`
	StartedAt  *string `json:"started_at,omitempty"`
	FinishedAt *string `json:"finished_at,omitempty"`
}

func formatRFC3339Ptr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func jobToPublic(j store.ReviewJob) jobPublic {
	return jobPublic{
		ID:         j.ID,
		FullName:   j.FullName,
		PRNumber:   j.PRNumber,
		Status:     j.Status,
		Error:      j.Error,
		CreatedAt:  j.CreatedAt.UTC().Format(time.RFC3339),
		StartedAt:  formatRFC3339Ptr(j.StartedAt),
		FinishedAt: formatRFC3339Ptr(j.FinishedAt),
	}
}

type processingJobBrief struct {
	ID        uint   `json:"id"`
	FullName  string `json:"full_name"`
	PRNumber  int    `json:"pr_number"`
	StartedAt string `json:"started_at,omitempty"`
}

func (s *Server) queueStatus(c *fiber.Ctx) error {
	counts, proc, err := s.store.ReviewQueueStatus()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	repos, err := s.store.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	out := fiber.Map{
		"queue": fiber.Map{
			"pending":    counts.Pending,
			"processing": counts.Processing,
			"completed":  counts.Completed,
			"failed":     counts.Failed,
		},
		"watched_repos": len(repos),
	}
	if proc != nil {
		brief := processingJobBrief{
			ID:       proc.ID,
			FullName: proc.FullName,
			PRNumber: proc.PRNumber,
		}
		if proc.StartedAt != nil {
			brief.StartedAt = proc.StartedAt.UTC().Format(time.RFC3339)
		}
		out["processing_job"] = brief
	} else {
		out["processing_job"] = nil
	}
	return c.JSON(out)
}

func parseJobListQuery(c *fiber.Ctx) (limit, offset int, status string, errMsg string) {
	limit = 50
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return 0, 0, "", "invalid limit"
		}
		if n > 500 {
			n = 500
		}
		limit = n
	}
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, 0, "", "invalid offset"
		}
		offset = n
	}
	status = strings.TrimSpace(strings.ToLower(c.Query("status")))
	if status == "" {
		return limit, offset, "", ""
	}
	switch status {
	case store.ReviewJobPending, store.ReviewJobProcessing, store.ReviewJobCompleted, store.ReviewJobFailed:
		return limit, offset, status, ""
	default:
		return 0, 0, "", "invalid status (use pending, processing, completed, or failed)"
	}
}

func (s *Server) listJobs(c *fiber.Ctx) error {
	limit, offset, status, errMsg := parseJobListQuery(c)
	if errMsg != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errMsg})
	}
	jobs, total, err := s.store.ListReviewJobs(limit, offset, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	out := make([]jobPublic, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, jobToPublic(j))
	}
	return c.JSON(fiber.Map{
		"jobs":   out,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) getJob(c *fiber.Ctx) error {
	id64, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id64 == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid job id"})
	}
	job, err := s.store.GetReviewJob(uint(id64))
	if err != nil {
		if store.IsRecordNotFound(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "job not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(jobToPublic(*job))
}
