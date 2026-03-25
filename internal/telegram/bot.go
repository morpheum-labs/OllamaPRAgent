package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/morpheum-labs/OllamaPRAgent/internal/ollama"
	"github.com/morpheum-labs/OllamaPRAgent/internal/review"
)

// Job is one PR review requested from Telegram.
type Job struct {
	ChatID   int64
	Username string
	Repo     string
	PR       int
}

type reportSnap struct {
	Repo string
	PR   int
	Text string
	At   time.Time
}

// Service runs the Telegram bot and review queue.
type Service struct {
	cfg   AppConfig
	store Store
	api   *tgbotapi.BotAPI
	log   *log.Logger

	mu       sync.Mutex
	paused   bool
	current  *Job
	queue    []Job
	runCtx   context.Context
	cancelRun context.CancelFunc

	reportsMu      sync.RWMutex
	reportByKey    map[string]reportSnap
	lastByChat     map[int64]reportSnap
}

// NewService wires the bot API, persistence, and in-memory state.
func NewService(cfg AppConfig, store Store, logger *log.Logger) (*Service, error) {
	if logger == nil {
		logger = log.Default()
	}
	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("telegram bot: %w", err)
	}
	api.Debug = false
	logger.Printf("authorized on telegram as %s", api.Self.UserName)

	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		cfg:         cfg,
		store:       store,
		api:         api,
		log:         logger,
		reportByKey: make(map[string]reportSnap),
		lastByChat:  make(map[int64]reportSnap),
		runCtx:      ctx,
		cancelRun:   cancel,
	}, nil
}

// Close stops background work (e.g. on shutdown).
func (s *Service) Close() {
	s.cancelRun()
}

func (s *Service) allowed(chatID int64) bool {
	if len(s.cfg.AllowedChatIDs) == 0 {
		return true
	}
	_, ok := s.cfg.AllowedChatIDs[chatID]
	return ok
}

func reportKey(chatID int64, repo string, pr int) string {
	return fmt.Sprintf("%d:%s:%d", chatID, repo, pr)
}

// Run starts long polling until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := s.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case upd := <-updates:
			if upd.Message == nil {
				continue
			}
			go s.dispatch(ctx, upd.Message)
		}
	}
}

func (s *Service) dispatch(ctx context.Context, msg *tgbotapi.Message) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Printf("telegram handler panic: %v", r)
		}
	}()
	if !s.allowed(msg.Chat.ID) {
		return
	}
	if msg.From == nil {
		return
	}
	text := ""
	if msg.Text != "" {
		text = msg.Text
	}
	if msg.IsCommand() {
		s.handleCommand(ctx, msg, text)
		return
	}
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.ID == s.api.Self.ID {
		s.handleFollowUp(ctx, msg.Chat.ID, text)
	}
}

func (s *Service) sendText(chatID int64, text string) {
	for _, chunk := range SplitMessageChunks(text, 4000) {
		m := tgbotapi.NewMessage(chatID, chunk)
		m.DisableWebPagePreview = true
		if _, err := s.api.Send(m); err != nil {
			s.log.Printf("send message: %v", err)
		}
	}
}

func (s *Service) handleCommand(ctx context.Context, msg *tgbotapi.Message, text string) {
	cmd := msg.Command()
	args := strings.TrimSpace(msg.CommandArguments())
	user := msg.From.UserName
	if user == "" {
		user = fmt.Sprintf("user:%d", msg.From.ID)
	}

	switch cmd {
	case "start", "help":
		s.sendText(msg.Chat.ID, helpText())
	case "ping":
		s.handlePing(msg.Chat.ID)
	case "status":
		s.handleStatus(msg.Chat.ID)
	case "tasks", "queue":
		s.handleTasks(msg.Chat.ID)
	case "review":
		s.handleReview(ctx, msg.Chat.ID, user, args)
	case "report":
		s.handleReport(msg.Chat.ID, args)
	case "watch":
		s.handleWatch(ctx, msg.Chat.ID, args)
	case "unwatch":
		s.handleUnwatch(ctx, msg.Chat.ID, args)
	case "pause":
		s.handlePause(msg.Chat.ID)
	case "resume":
		s.handleResume(ctx, msg.Chat.ID)
	case "config":
		s.handleConfig(ctx, msg.Chat.ID, args)
	case "feedback":
		s.handleFeedbackCmd(ctx, msg.Chat.ID, args)
	case "daily":
		s.handleDaily(ctx, msg.Chat.ID)
	default:
		s.sendText(msg.Chat.ID, "Unknown command. Try /help.")
	}
}

func helpText() string {
	return `Hi — I'm your Ollama PR Agent. I review pull requests using your local Ollama model and can post back to Gitea (or use file/git providers via env).

Commands:
/help — this message
/ping — Ollama health and model hint
/status — paused state and current review
/tasks — queued reviews
/review <n> | owner/repo#n — run a review
/report <n> | owner/repo#n — last cached review text for this chat
/watch owner/repo — persist a watch list (for future webhook notifications)
/unwatch owner/repo
/pause — pause starting new reviews (in-flight finishes)
/resume — drain the queue again
/config — show settings; /config model <name> — per-chat model override
/feedback <message> — ask about the last review (Ollama chat)
/daily — reviews completed today (UTC) for this chat

Tip: reply to one of my messages with a question to continue the conversation about that review.`
}

func (s *Service) handlePing(chatID int64) {
	url := s.cfg.Review.OllamaURL
	if err := ollama.Ping(url); err != nil {
		s.sendText(chatID, fmt.Sprintf("Ollama check failed: %v", err))
		return
	}
	model := s.cfg.Review.OllamaModel
	loaded := ollama.ModelLoaded(url, model)
	extra := "model not listed in /api/tags (may still work if pulled on demand)"
	if loaded {
		extra = "model appears in /api/tags"
	}
	s.sendText(chatID, fmt.Sprintf("Pong! Ollama is up at %s. Using %q — %s.", url, model, extra))
}

func (s *Service) handleStatus(chatID int64) {
	s.mu.Lock()
	paused := s.paused
	var cur *Job
	if s.current != nil {
		j := *s.current
		cur = &j
	}
	qn := len(s.queue)
	s.mu.Unlock()

	var b strings.Builder
	if paused {
		b.WriteString("Agent is paused. No new reviews start until /resume.\n")
	} else {
		b.WriteString("Agent is running.\n")
	}
	if cur != nil {
		b.WriteString(fmt.Sprintf("Current: PR #%d in %s (requested by @%s).\n", cur.PR, cur.Repo, cur.Username))
	} else {
		b.WriteString("Current: idle.\n")
	}
	b.WriteString(fmt.Sprintf("Queued: %d.", qn))
	s.sendText(chatID, b.String())
}

func (s *Service) handleTasks(chatID int64) {
	s.mu.Lock()
	var cur *Job
	if s.current != nil {
		j := *s.current
		cur = &j
	}
	q := append([]Job(nil), s.queue...)
	s.mu.Unlock()

	var b strings.Builder
	b.WriteString("Tasks:\n")
	if cur != nil {
		b.WriteString(fmt.Sprintf("- in progress: %s#%d (@%s)\n", cur.Repo, cur.PR, cur.Username))
	} else {
		b.WriteString("- in progress: none\n")
	}
	if len(q) == 0 {
		b.WriteString("- queue: empty\n")
	} else {
		for i, j := range q {
			b.WriteString(fmt.Sprintf("- queued %d: %s#%d (@%s)\n", i+1, j.Repo, j.PR, j.Username))
		}
	}
	s.sendText(chatID, b.String())
}

func (s *Service) handleReview(ctx context.Context, chatID int64, user, args string) {
	repo, pr, err := ParseReviewTarget(args, s.cfg.Review.DefaultRepo)
	if err != nil {
		s.sendText(chatID, err.Error())
		return
	}
	job := Job{ChatID: chatID, Username: user, Repo: repo, PR: pr}

	s.mu.Lock()
	if s.paused {
		s.queue = append(s.queue, job)
		s.mu.Unlock()
		s.sendText(chatID, fmt.Sprintf("Agent is paused — queued review for %s#%d. Use /resume to process.", repo, pr))
		return
	}
	if s.current != nil {
		s.queue = append(s.queue, job)
		s.mu.Unlock()
		s.sendText(chatID, fmt.Sprintf("Busy on another review — queued %s#%d. I’ll run it next.", repo, pr))
		return
	}
	s.current = &job
	s.mu.Unlock()

	s.sendText(chatID, fmt.Sprintf("Starting review for %s#%d…", repo, pr))
	go s.executeReview(ctx, job)
}

func (s *Service) handleReport(chatID int64, args string) {
	repo, pr, err := ParseReviewTarget(args, s.cfg.Review.DefaultRepo)
	if err != nil {
		s.sendText(chatID, err.Error())
		return
	}
	key := reportKey(chatID, repo, pr)
	s.reportsMu.RLock()
	snap, ok := s.reportByKey[key]
	s.reportsMu.RUnlock()
	if !ok {
		s.sendText(chatID, "No cached report for that PR in this chat yet. Run /review first.")
		return
	}
	s.sendText(chatID, fmt.Sprintf("Report for %s#%d (saved %s ago):\n\n%s", repo, pr, time.Since(snap.At).Round(time.Second), snap.Text))
}

func (s *Service) handleWatch(ctx context.Context, chatID int64, args string) {
	repo, err := ParseRepoArg(args)
	if err != nil {
		s.sendText(chatID, err.Error())
		return
	}
	if err := s.store.AddWatch(ctx, chatID, repo); err != nil {
		s.sendText(chatID, fmt.Sprintf("Could not save watch: %v", err))
		return
	}
	s.sendText(chatID, fmt.Sprintf("Watching %s — stored for future automation; use /review to run a review now.", repo))
}

func (s *Service) handleUnwatch(ctx context.Context, chatID int64, args string) {
	repo, err := ParseRepoArg(args)
	if err != nil {
		s.sendText(chatID, err.Error())
		return
	}
	if err := s.store.RemoveWatch(ctx, chatID, repo); err != nil {
		s.sendText(chatID, fmt.Sprintf("Could not remove watch: %v", err))
		return
	}
	s.sendText(chatID, fmt.Sprintf("Stopped watching %s.", repo))
}

func (s *Service) handlePause(chatID int64) {
	s.mu.Lock()
	s.paused = true
	s.mu.Unlock()
	s.sendText(chatID, "Agent paused. In-flight review will finish; no new ones start until /resume.")
}

func (s *Service) handleResume(ctx context.Context, chatID int64) {
	s.mu.Lock()
	s.paused = false
	s.mu.Unlock()
	s.sendText(chatID, "Agent resumed.")
	s.pumpQueue(ctx)
}

func (s *Service) handleConfig(ctx context.Context, chatID int64, args string) {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		override, _ := s.store.GetChatModel(ctx, chatID)
		if override == "" {
			override = "(default)"
		}
		watches, _ := s.store.ListWatches(ctx, chatID)
		watchLine := "(none)"
		if len(watches) > 0 {
			watchLine = strings.Join(watches, ", ")
		}
		msg := fmt.Sprintf(`Settings for this chat:
- Ollama URL: %s
- Default model: %s
- Per-chat model override: %s
- Provider (ORB_PROVIDER): %s
- Default repo (ORB_REPO_NAME): %s
- Post comment: %v
- Watched repos: %s

Use: /config model <name> or /config model clear`,
			s.cfg.Review.OllamaURL,
			s.cfg.Review.OllamaModel,
			override,
			s.cfg.Review.Provider,
			emptyFallback(s.cfg.Review.DefaultRepo, "(unset)"),
			s.cfg.Review.PostComment,
			watchLine)
		s.sendText(chatID, msg)
		return
	}
	if fields[0] == "model" {
		if len(fields) == 1 {
			s.sendText(chatID, "Usage: /config model <name> or /config model clear")
			return
		}
		if fields[1] == "clear" {
			_ = s.store.SetChatModel(ctx, chatID, "")
			s.sendText(chatID, "Per-chat model cleared; using default from environment.")
			return
		}
		name := strings.TrimSpace(strings.Join(fields[1:], " "))
		if err := s.store.SetChatModel(ctx, chatID, name); err != nil {
			s.sendText(chatID, fmt.Sprintf("Could not save model: %v", err))
			return
		}
		s.sendText(chatID, fmt.Sprintf("Per-chat model set to %q.", name))
		return
	}
	s.sendText(chatID, "Unknown /config usage. Try /config with no args.")
}

func (s *Service) handleFeedbackCmd(ctx context.Context, chatID int64, args string) {
	args = strings.TrimSpace(args)
	if args == "" {
		s.sendText(chatID, "Usage: /feedback <your question or note about the last review>")
		return
	}
	s.answerFollowUp(ctx, chatID, args)
}

func (s *Service) handleFollowUp(ctx context.Context, chatID int64, question string) {
	question = strings.TrimSpace(question)
	if question == "" {
		return
	}
	s.answerFollowUp(ctx, chatID, question)
}

func (s *Service) answerFollowUp(ctx context.Context, chatID int64, question string) {
	s.reportsMu.RLock()
	snap, ok := s.lastByChat[chatID]
	s.reportsMu.RUnlock()
	if !ok {
		s.sendText(chatID, "I don’t have a recent review in this chat. Run /review first, or reply to my review message.")
		return
	}
	model, _ := s.store.GetChatModel(ctx, chatID)
	if model == "" {
		model = s.cfg.Review.OllamaModel
	}
	prompt := fmt.Sprintf("You are a senior engineer. The user is discussing a prior pull request review.\n\nRepository: %s\nPR: #%d\n\nReview summary/text:\n%s\n\nUser message:\n%s\n\nAnswer clearly and concisely. If you need code, use markdown fences.",
		snap.Repo, snap.PR, snap.Text, question)
	resp, err := ollama.SendPrompt(ollama.RequestOptions{
		URL:         s.cfg.Review.OllamaURL,
		Model:       model,
		Prompt:      prompt,
		Temperature: s.cfg.Review.OllamaTemp,
		TopP:        s.cfg.Review.OllamaTopP,
		NumTokens:   s.cfg.Review.OllamaMaxTok,
	})
	if err != nil {
		s.sendText(chatID, fmt.Sprintf("Ollama error: %v", err))
		return
	}
	s.sendText(chatID, resp)
}

func (s *Service) handleDaily(ctx context.Context, chatID int64) {
	n, err := s.store.GetDailyReviews(ctx, chatID, time.Now().UTC())
	if err != nil {
		s.sendText(chatID, fmt.Sprintf("Could not read stats: %v", err))
		return
	}
	s.sendText(chatID, fmt.Sprintf("Today (UTC) I completed %d review(s) in this chat.", n))
}

func (s *Service) pumpQueue(ctx context.Context) {
	s.mu.Lock()
	if s.paused || s.current != nil || len(s.queue) == 0 {
		s.mu.Unlock()
		return
	}
	job := s.queue[0]
	s.queue = s.queue[1:]
	s.current = &job
	s.mu.Unlock()
	s.sendText(job.ChatID, fmt.Sprintf("Starting queued review for %s#%d…", job.Repo, job.PR))
	go s.executeReview(ctx, job)
}

func (s *Service) finishJobAndPump(ctx context.Context) {
	s.mu.Lock()
	s.current = nil
	s.mu.Unlock()
	s.pumpQueue(ctx)
}

func (s *Service) executeReview(ctx context.Context, job Job) {
	defer s.finishJobAndPump(ctx)

	chatModel, err := s.store.GetChatModel(ctx, job.ChatID)
	if err != nil {
		s.log.Printf("chat model: %v", err)
		chatModel = ""
	}
	opts := s.cfg.Review.BuildReviewOptions(job.Repo, job.PR, chatModel, func(msg string) {
		s.sendText(job.ChatID, msg)
	})

	res, err := review.Run(ctx, opts)
	if err != nil {
		s.sendText(job.ChatID, fmt.Sprintf("Review failed for %s#%d: %v", job.Repo, job.PR, err))
		return
	}

	snap := reportSnap{Repo: job.Repo, PR: job.PR, Text: res.Review, At: time.Now().UTC()}
	s.reportsMu.Lock()
	s.reportByKey[reportKey(job.ChatID, job.Repo, job.PR)] = snap
	s.lastByChat[job.ChatID] = snap
	s.reportsMu.Unlock()

	if err := s.store.IncDailyReviews(ctx, job.ChatID, time.Now().UTC()); err != nil {
		s.log.Printf("daily stats: %v", err)
	}

	s.sendText(job.ChatID, fmt.Sprintf("Review complete for %s#%d:\n\n%s", job.Repo, job.PR, res.Review))
}

func emptyFallback(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
