package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// WatchedRepo is a GitHub repository registered for webhook delivery.
type WatchedRepo struct {
	ID        uint `gorm:"primaryKey"`
	FullName  string `gorm:"uniqueIndex;not null"` // owner/repo (normalized lowercase)
	Secret    string `gorm:"not null"`             // HMAC secret for X-Hub-Signature-256
	CreatedAt time.Time
}

// Store persists watched repositories (pure Go SQLite via glebarez).
type Store struct {
	db *gorm.DB
}

// New opens the database and runs migrations.
func New(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.AutoMigrate(&WatchedRepo{}, &ReviewJob{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// DB exposes the underlying handle for tests or advanced use.
func (s *Store) DB() *gorm.DB {
	return s.db
}

func normalizeFullName(name string) (string, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "", fmt.Errorf("empty repo")
	}
	parts := strings.Split(name, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(parts[0], "/") {
		return "", fmt.Errorf("repo must be owner/name")
	}
	return name, nil
}

// Add registers a repository and stores the webhook secret.
func (s *Store) Add(repoFullName, secret string) error {
	fn, err := normalizeFullName(repoFullName)
	if err != nil {
		return err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return fmt.Errorf("empty secret")
	}
	r := &WatchedRepo{FullName: fn, Secret: secret}
	if err := s.db.Create(r).Error; err != nil {
		return err
	}
	return nil
}

// Remove deletes a watched repository by full name.
func (s *Store) Remove(fullName string) error {
	fn, err := normalizeFullName(fullName)
	if err != nil {
		return err
	}
	res := s.db.Where("full_name = ?", fn).Delete(&WatchedRepo{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// List returns all watched repositories (including secrets — for internal use only).
func (s *Store) List() ([]WatchedRepo, error) {
	var repos []WatchedRepo
	err := s.db.Order("full_name").Find(&repos).Error
	return repos, err
}

// GetByName returns a watched repo or gorm.ErrRecordNotFound.
func (s *Store) GetByName(fullName string) (*WatchedRepo, error) {
	fn, err := normalizeFullName(fullName)
	if err != nil {
		return nil, err
	}
	var r WatchedRepo
	if err := s.db.Where("full_name = ?", fn).First(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

// IsRecordNotFound reports whether err is a missing row.
func IsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
