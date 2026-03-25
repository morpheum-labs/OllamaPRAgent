package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using a local SQLite file (pure Go driver, no CGO).
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLite opens (and migrates) a SQLite database at path (e.g. /data/telegram.db).
func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS watches (
			chat_id INTEGER NOT NULL,
			repo TEXT NOT NULL,
			PRIMARY KEY (chat_id, repo)
		)`,
		`CREATE TABLE IF NOT EXISTS chat_settings (
			chat_id INTEGER PRIMARY KEY,
			ollama_model TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS daily_stats (
			chat_id INTEGER NOT NULL,
			day TEXT NOT NULL,
			reviews INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (chat_id, day)
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// Close releases the database handle.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) AddWatch(ctx context.Context, chatID int64, repo string) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return fmt.Errorf("empty repo")
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO watches(chat_id, repo) VALUES(?,?)`, chatID, repo)
	return err
}

func (s *SQLiteStore) RemoveWatch(ctx context.Context, chatID int64, repo string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM watches WHERE chat_id=? AND repo=?`, chatID, strings.TrimSpace(repo))
	return err
}

func (s *SQLiteStore) ListWatches(ctx context.Context, chatID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT repo FROM watches WHERE chat_id=? ORDER BY repo`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ListAllWatches(ctx context.Context) (map[int64][]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT chat_id, repo FROM watches ORDER BY chat_id, repo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64][]string)
	for rows.Next() {
		var chat int64
		var repo string
		if err := rows.Scan(&chat, &repo); err != nil {
			return nil, err
		}
		m[chat] = append(m[chat], repo)
	}
	return m, rows.Err()
}

func (s *SQLiteStore) GetChatModel(ctx context.Context, chatID int64) (string, error) {
	var m sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT ollama_model FROM chat_settings WHERE chat_id=?`, chatID).Scan(&m)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !m.Valid {
		return "", nil
	}
	return m.String, nil
}

func (s *SQLiteStore) SetChatModel(ctx context.Context, chatID int64, model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM chat_settings WHERE chat_id=?`, chatID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO chat_settings(chat_id, ollama_model) VALUES(?,?)
		ON CONFLICT(chat_id) DO UPDATE SET ollama_model=excluded.ollama_model
	`, chatID, model)
	return err
}

func dayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func (s *SQLiteStore) IncDailyReviews(ctx context.Context, chatID int64, day time.Time) error {
	d := dayKey(day)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_stats(chat_id, day, reviews) VALUES(?,?,1)
		ON CONFLICT(chat_id, day) DO UPDATE SET reviews = reviews + 1
	`, chatID, d)
	return err
}

func (s *SQLiteStore) GetDailyReviews(ctx context.Context, chatID int64, day time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT reviews FROM daily_stats WHERE chat_id=? AND day=?`, chatID, dayKey(day)).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}
