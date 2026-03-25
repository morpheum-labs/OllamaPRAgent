package telegram

import (
	"context"
	"time"
)

// Store persists watches, per-chat model overrides, and daily review counts.
type Store interface {
	AddWatch(ctx context.Context, chatID int64, repo string) error
	RemoveWatch(ctx context.Context, chatID int64, repo string) error
	ListWatches(ctx context.Context, chatID int64) ([]string, error)
	ListAllWatches(ctx context.Context) (map[int64][]string, error)

	GetChatModel(ctx context.Context, chatID int64) (string, error)
	SetChatModel(ctx context.Context, chatID int64, model string) error

	IncDailyReviews(ctx context.Context, chatID int64, day time.Time) error
	GetDailyReviews(ctx context.Context, chatID int64, day time.Time) (int, error)
}
