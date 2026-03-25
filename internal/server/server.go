package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/morpheum-labs/OllamaPRAgent/internal/config"
	"github.com/morpheum-labs/OllamaPRAgent/internal/gitprovider"
	"github.com/morpheum-labs/OllamaPRAgent/internal/ollama"
	"github.com/morpheum-labs/OllamaPRAgent/internal/review"
	"github.com/morpheum-labs/OllamaPRAgent/internal/store"
)

// Server is the HTTP API and GitHub webhook front-end.
type Server struct {
	app    *fiber.App
	store  *store.Store
	cfg    config.Server
	logger *log.Logger

	// reviewWake signals the durable queue worker (buffer 1 coalesces bursts).
	reviewWake chan struct{}
}

// New constructs a Fiber app with routes registered.
func New(st *store.Store, cfg config.Server, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	s := &Server{
		app:        app,
		store:      st,
		cfg:        cfg,
		logger:     logger,
		reviewWake: make(chan struct{}, 1),
	}
	go s.reviewWorker()

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/webhook", s.handleGitHubWebhook)

	api := app.Group("/api", s.requireAdmin)
	api.Get("/status", s.queueStatus)
	api.Get("/jobs", s.listJobs)
	api.Get("/jobs/:id", s.getJob)
	api.Post("/repos", s.addRepo)
	api.Delete("/repos/:owner/:repo", s.removeRepo)
	api.Get("/repos", s.listRepos)

	return s
}

func (s *Server) requireAdmin(c *fiber.Ctx) error {
	if s.cfg.AdminToken == "" {
		return c.Next()
	}
	h := strings.TrimSpace(c.Get("Authorization"))
	if h != "Bearer "+s.cfg.AdminToken {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	return c.Next()
}

type addRepoRequest struct {
	Repo string `json:"repo"`
}

type repoPublic struct {
	ID        uint   `json:"id"`
	FullName  string `json:"full_name"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) addRepo(c *fiber.Ctx) error {
	var req addRepoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid json"})
	}
	fn, err := normalizeRepoInput(req.Repo)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	secret := "whsec_" + uuid.New().String()

	_, getErr := s.store.GetByName(fn)
	if getErr == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "repo already registered"})
	}
	if !store.IsRecordNotFound(getErr) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": getErr.Error()})
	}

	if err := s.store.Add(fn, secret); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	webhookURL := "(set SERVER_PUBLIC_URL to show a full webhook URL)"
	if s.cfg.PublicURL != "" {
		webhookURL = s.cfg.PublicURL + "/webhook"
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":      "added",
		"repo":        fn,
		"secret":      secret,
		"webhook_url": webhookURL,
		"message":     "Add a GitHub webhook: Content type application/json, URL above, secret above; events: Pull requests",
	})
}

func (s *Server) removeRepo(c *fiber.Ctx) error {
	full := c.Params("owner") + "/" + c.Params("repo")
	if err := s.store.Remove(full); err != nil {
		if store.IsRecordNotFound(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "repo not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "removed", "repo": strings.ToLower(strings.TrimSpace(full))})
}

func (s *Server) listRepos(c *fiber.Ctx) error {
	repos, err := s.store.List()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	out := make([]repoPublic, 0, len(repos))
	for _, r := range repos {
		out = append(out, repoPublic{
			ID:        r.ID,
			FullName:  r.FullName,
			CreatedAt: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return c.JSON(out)
}

type pullRequestWebhook struct {
	Action string `json:"action"`
	Repo   struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	PullRequest struct {
		Number int `json:"number"`
	} `json:"pull_request"`
}

func (s *Server) handleGitHubWebhook(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Get("X-GitHub-Delivery")) == "" && c.Get("X-GitHub-Event") == "" {
		// Likely not GitHub; reject noise
		return c.SendStatus(fiber.StatusBadRequest)
	}

	event := c.Get("X-GitHub-Event")
	if event == "ping" {
		return c.SendStatus(fiber.StatusOK)
	}
	if event != "pull_request" {
		return c.SendStatus(fiber.StatusOK)
	}

	body := c.Body()
	var payload pullRequestWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid json")
	}

	fullName := strings.TrimSpace(payload.Repo.FullName)
	if fullName == "" {
		return c.SendStatus(fiber.StatusOK)
	}

	wr, err := s.store.GetByName(fullName)
	if err != nil {
		if store.IsRecordNotFound(err) {
			s.logger.Printf("webhook ignored (not watched): %s", fullName)
			return c.SendStatus(fiber.StatusOK)
		}
		s.logger.Printf("store get %s: %v", fullName, err)
		return c.Status(fiber.StatusInternalServerError).SendString("store error")
	}

	sig := c.Get("X-Hub-Signature-256")
	if !verifyGitHubSignature256(body, wr.Secret, sig) {
		s.logger.Printf("invalid webhook signature for %s", fullName)
		return c.Status(fiber.StatusForbidden).SendString("bad signature")
	}

	if payload.Action != "opened" && payload.Action != "synchronize" && payload.Action != "reopened" && payload.Action != "ready_for_review" {
		return c.SendStatus(fiber.StatusOK)
	}

	prNum := payload.PullRequest.Number
	if prNum <= 0 {
		return c.SendStatus(fiber.StatusOK)
	}

	jobID, enqErr := s.store.EnqueueReviewJob(fullName, prNum)
	if enqErr != nil {
		s.logger.Printf("enqueue review %s#%d: %v", fullName, prNum, enqErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "queue failed"})
	}

	s.logger.Printf("queued review job %d %s#%d (action=%s)", jobID, fullName, prNum, payload.Action)

	s.notifyReviewQueue()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted", "job_id": jobID})
}

func (s *Server) reviewWorker() {
	if err := s.store.ResetStaleProcessingJobs(); err != nil {
		s.logger.Printf("review queue: reset stale processing jobs: %v", err)
	}
	s.notifyReviewQueue()
	for {
		for {
			job, err := s.store.ClaimNextReviewJob()
			if err != nil {
				s.logger.Printf("review queue: claim: %v", err)
				break
			}
			if job == nil {
				break
			}
			s.runGitHubReview(job.FullName, job.PRNumber, job.ID)
		}
		<-s.reviewWake
	}
}

func (s *Server) notifyReviewQueue() {
	select {
	case s.reviewWake <- struct{}{}:
	default:
	}
}

func (s *Server) runGitHubReview(fullName string, prNum int, jobID uint) {
	ctx := context.Background()
	opts := review.Options{
		ProviderConfig: gitprovider.Config{
			ProviderType:  gitprovider.GitHubProviderType,
			RepoName:      fullName,
			PRNumber:      prNum,
			GitHubToken:   s.cfg.GitHubPAT,
			GitHubBaseURL: s.cfg.GitHubBaseURL,
		},
		RepoRoot:       s.cfg.RepoRoot,
		PromptTemplate: s.cfg.PromptTemplate,
		PostComment:    s.cfg.PostComment,
		Ollama: ollama.RequestOptions{
			URL:         s.cfg.OllamaURL,
			Model:       s.cfg.OllamaModel,
			Temperature: s.cfg.OllamaTemp,
			TopP:        s.cfg.OllamaTopP,
			NumTokens:   s.cfg.OllamaMaxTok,
		},
		OnProgress: func(msg string) {
			s.logger.Printf("[%s#%d] %s", fullName, prNum, msg)
		},
	}

	res, err := review.Run(ctx, opts)
	if err != nil {
		if finErr := s.store.FinishReviewJob(jobID, err); finErr != nil {
			s.logger.Printf("finish failed job %d: %v", jobID, finErr)
		}
		s.logger.Printf("review failed %s#%d: %v", fullName, prNum, err)
		return
	}
	if finErr := s.store.FinishReviewJob(jobID, nil); finErr != nil {
		s.logger.Printf("finish completed job %d: %v", jobID, finErr)
	}
	s.logger.Printf("review done %s#%d (%d chars)", fullName, prNum, len(res.Review))
}

// Listen serves HTTP on hostPort, e.g. ":8080".
func (s *Server) Listen(hostPort string) error {
	s.logger.Printf("Ollama PR Agent HTTP listening on %s (webhook POST /webhook)", hostPort)
	return s.app.Listen(hostPort)
}

func normalizeRepoInput(repo string) (string, error) {
	repo = strings.TrimSpace(strings.ToLower(repo))
	if repo == "" {
		return "", fmt.Errorf("repo is required")
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("repo must be owner/name")
	}
	return repo, nil
}
