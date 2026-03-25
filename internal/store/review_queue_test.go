package store

import (
	"path/filepath"
	"testing"
)

func TestReviewQueueEnqueueClaimFinish(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := New(filepath.Join(dir, "q.db"))
	if err != nil {
		t.Fatal(err)
	}

	id1, err := st.EnqueueReviewJob("Acme/Widget", 42)
	if err != nil || id1 == 0 {
		t.Fatalf("enqueue: id=%d err=%v", id1, err)
	}

	job, err := st.ClaimNextReviewJob()
	if err != nil || job == nil {
		t.Fatalf("claim: job=%v err=%v", job, err)
	}
	if job.FullName != "acme/widget" || job.PRNumber != 42 || job.Status != ReviewJobProcessing {
		t.Fatalf("unexpected job: %+v", job)
	}

	if err := st.FinishReviewJob(job.ID, nil); err != nil {
		t.Fatal(err)
	}
	var stored ReviewJob
	if err := st.DB().First(&stored, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != ReviewJobCompleted {
		t.Fatalf("want completed, got %q", stored.Status)
	}
}

func TestReviewQueueCoalescePending(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := New(filepath.Join(dir, "q.db"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = st.EnqueueReviewJob("owner/repo", 1)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := st.EnqueueReviewJob("owner/repo", 1)
	if err != nil {
		t.Fatal(err)
	}

	var n int64
	st.DB().Model(&ReviewJob{}).Where("status = ?", ReviewJobPending).Count(&n)
	if n != 1 {
		t.Fatalf("want 1 pending row, got %d", n)
	}

	job, err := st.ClaimNextReviewJob()
	if err != nil || job == nil || job.ID != id2 {
		t.Fatalf("claim id want %d got %+v err=%v", id2, job, err)
	}
}

func TestResetStaleProcessingJobs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := New(filepath.Join(dir, "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	id, err := st.EnqueueReviewJob("o/r", 9)
	if err != nil {
		t.Fatal(err)
	}
	job, err := st.ClaimNextReviewJob()
	if err != nil || job == nil {
		t.Fatal(err)
	}
	if job.ID != id {
		t.Fatalf("id mismatch")
	}

	if err := st.ResetStaleProcessingJobs(); err != nil {
		t.Fatal(err)
	}
	var stored ReviewJob
	if err := st.DB().First(&stored, id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != ReviewJobPending {
		t.Fatalf("want pending after reset, got %q", stored.Status)
	}
}

func TestListReviewJobsAndStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := New(filepath.Join(dir, "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.EnqueueReviewJob("a/b", 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.EnqueueReviewJob("c/d", 2)
	if err != nil {
		t.Fatal(err)
	}

	jobs, total, err := st.ListReviewJobs(10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got total=%d len=%d", total, len(jobs))
	}

	counts, proc, err := st.ReviewQueueStatus()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Pending != 2 || proc != nil {
		t.Fatalf("unexpected status: %+v proc=%v", counts, proc)
	}

	j, err := st.ClaimNextReviewJob()
	if err != nil || j == nil {
		t.Fatalf("claim: %v", err)
	}
	counts, proc, err = st.ReviewQueueStatus()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Pending != 1 || counts.Processing != 1 || proc == nil || proc.ID != j.ID {
		t.Fatalf("after claim: counts=%+v proc=%v", counts, proc)
	}
}
