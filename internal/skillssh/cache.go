package skillssh

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const cacheVersion = 2
const maxCacheBytes = 32 << 20

type cacheDocument struct {
	Version  int               `json:"version"`
	Pages    map[string]Page   `json:"pages"`
	Searches map[string]Page   `json:"searches,omitempty"`
	Details  map[string]Detail `json:"details"`
}

type cacheStore struct{ path string }

func (s cacheStore) load() (cacheDocument, error) {
	document := emptyCache()
	info, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return document, nil
	}
	if err != nil {
		return document, fmt.Errorf("inspect skills.sh cache: %w", err)
	}
	if info.Size() > maxCacheBytes {
		return document, fmt.Errorf("skills.sh cache exceeds %d bytes", maxCacheBytes)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return document, fmt.Errorf("read skills.sh cache: %w", err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return emptyCache(), fmt.Errorf("decode skills.sh cache: %w", err)
	}
	if document.Version != 1 && document.Version != cacheVersion {
		return emptyCache(), fmt.Errorf("unsupported skills.sh cache version %d", document.Version)
	}
	document.Version = cacheVersion
	if document.Pages == nil {
		document.Pages = map[string]Page{}
	}
	if document.Details == nil {
		document.Details = map[string]Detail{}
	}
	if document.Searches == nil {
		document.Searches = map[string]Page{}
	}
	if err := validateCachedDocument(&document); err != nil {
		return emptyCache(), fmt.Errorf("validate skills.sh cache: %w", err)
	}
	return document, nil
}

func (s cacheStore) save(document cacheDocument) error {
	document.Version = cacheVersion
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create skills.sh cache directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("secure skills.sh cache directory: %w", err)
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode skills.sh cache: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), "catalog-*.json")
	if err != nil {
		return fmt.Errorf("create skills.sh cache temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("secure skills.sh cache: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write skills.sh cache: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync skills.sh cache: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close skills.sh cache: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace skills.sh cache: %w", err)
	}
	committed = true
	return nil
}

func emptyCache() cacheDocument {
	return cacheDocument{Version: cacheVersion, Pages: map[string]Page{}, Searches: map[string]Page{}, Details: map[string]Detail{}}
}

// SanitizeCache upgrades a legacy catalog cache and removes persisted search
// terms. Missing caches are left untouched.
func SanitizeCache(path string) error {
	store := cacheStore{path: path}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect skills.sh cache for privacy migration: %w", err)
	}
	document, err := store.load()
	if err != nil {
		return err
	}
	document.Searches = map[string]Page{}
	return store.save(document)
}

func pageCacheKey(view View, page int) string { return fmt.Sprintf("%s:%d", view, page) }

func validateCachedDocument(document *cacheDocument) error {
	for key, page := range document.Pages {
		if !validView(page.View) || page.Page < 0 || page.Total < 0 || len(page.Skills) > 200 {
			return fmt.Errorf("invalid cached leaderboard %q", key)
		}
		if err := validateCachedSkills(page.Skills); err != nil {
			return fmt.Errorf("leaderboard %q: %w", key, err)
		}
		document.Pages[key] = page
	}
	for key, page := range document.Searches {
		if len(page.Skills) > 100 || page.Total < 0 {
			return fmt.Errorf("invalid cached search %q", key)
		}
		if err := validateCachedSkills(page.Skills); err != nil {
			return fmt.Errorf("search %q: %w", key, err)
		}
		document.Searches[key] = page
	}
	for key, detail := range document.Details {
		if key != detail.Skill.ID {
			return fmt.Errorf("detail key %q does not match skill ID", key)
		}
		canonicalizeSkill(&detail.Skill)
		if err := validateSkill(detail.Skill); err != nil {
			return fmt.Errorf("detail %q: %w", key, err)
		}
		detail.Description = truncateRunes(detail.Description, 4000)
		detail.AuditStatus = "external-only"
		document.Details[key] = detail
	}
	return nil
}

func validateCachedSkills(skills []Skill) error {
	for index := range skills {
		canonicalizeSkill(&skills[index])
		if err := validateSkill(skills[index]); err != nil {
			return err
		}
	}
	return nil
}

func pruneOldestPages(pages map[string]Page, limit int) {
	for len(pages) > limit {
		oldestKey := ""
		var oldest time.Time
		for key, page := range pages {
			if oldestKey == "" || page.FetchedAt.Before(oldest) {
				oldestKey, oldest = key, page.FetchedAt
			}
		}
		delete(pages, oldestKey)
	}
}

func pruneOldestDetails(details map[string]Detail, limit int) {
	for len(details) > limit {
		oldestKey := ""
		var oldest time.Time
		for key, detail := range details {
			if oldestKey == "" || detail.FetchedAt.Before(oldest) {
				oldestKey, oldest = key, detail.FetchedAt
			}
		}
		delete(details, oldestKey)
	}
}
