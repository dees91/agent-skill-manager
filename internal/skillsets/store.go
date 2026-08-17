// Package skillsets persists reusable task-oriented skill recipes without
// coupling them to the ownership and toggle manifest.
package skillsets

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

const (
	formatVersion       = 1
	privateDirMode      = 0o700
	privateFileMode     = 0o600
	maxBackups          = 10
	maxBackupAge        = 30 * 24 * time.Hour
	backupTimeFormat    = "20060102T150405.000000000Z"
	skillSetIDByteCount = 16
)

// Set is one saved, tool-agnostic collection of skill basenames.
type Set struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Skills      []string  `json:"skills"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// File is the versioned on-disk skill-sets.json shape.
type File struct {
	Version int   `json:"version"`
	Sets    []Set `json:"sets"`
}

// Store owns atomic persistence and bounded backups for Skill Sets.
type Store struct {
	paths paths.Paths
	now   func() time.Time
	newID func() (string, error)
}

// New creates a store rooted in the provided synthetic or real home.
func New(p paths.Paths) Store {
	return Store{paths: p, now: time.Now, newID: randomID}
}

// Load reads and validates the Skill Set file. A missing file is empty.
func (s Store) Load() (File, error) {
	if err := secureExistingFile(s.paths.SkillSetsFile); err != nil {
		return File{}, err
	}
	data, err := os.ReadFile(s.paths.SkillSetsFile)
	if errors.Is(err, os.ErrNotExist) {
		return emptyFile(), nil
	}
	if err != nil {
		return File{}, fmt.Errorf("load Skill Sets %s: %w", s.paths.SkillSetsFile, err)
	}
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("decode Skill Sets %s: %w", s.paths.SkillSetsFile, err)
	}
	if file.Version != formatVersion {
		return File{}, fmt.Errorf("unsupported Skill Sets version %d; this binary supports version %d", file.Version, formatVersion)
	}
	normalized, err := normalizeFile(file)
	if err != nil {
		return File{}, fmt.Errorf("validate Skill Sets %s: %w", s.paths.SkillSetsFile, err)
	}
	return normalized, nil
}

// Create validates and persists one new set.
func (s Store) Create(name, description string, skills []string) (Set, error) {
	file, err := s.Load()
	if err != nil {
		return Set{}, err
	}
	name = strings.TrimSpace(name)
	if duplicateName(file.Sets, "", name) {
		return Set{}, fmt.Errorf("a Skill Set named %q already exists", name)
	}
	id, err := s.newID()
	if err != nil {
		return Set{}, err
	}
	now := s.now().UTC()
	created := Set{ID: id, Name: name, Description: strings.TrimSpace(description), Skills: skills, CreatedAt: now, UpdatedAt: now}
	created, err = normalizeSet(created)
	if err != nil {
		return Set{}, err
	}
	file.Sets = append(file.Sets, created)
	file, err = normalizeFile(file)
	if err != nil {
		return Set{}, err
	}
	if err := s.saveMutation(file); err != nil {
		return Set{}, err
	}
	return created, nil
}

// Update replaces the user-editable fields for one stable set ID.
func (s Store) Update(id, name, description string, skills []string) (Set, error) {
	file, err := s.Load()
	if err != nil {
		return Set{}, err
	}
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if duplicateName(file.Sets, id, name) {
		return Set{}, fmt.Errorf("a Skill Set named %q already exists", name)
	}
	for index := range file.Sets {
		if file.Sets[index].ID != id {
			continue
		}
		updated := file.Sets[index]
		updated.Name = name
		updated.Description = strings.TrimSpace(description)
		updated.Skills = skills
		updated.UpdatedAt = s.now().UTC()
		updated, err = normalizeSet(updated)
		if err != nil {
			return Set{}, err
		}
		file.Sets[index] = updated
		file, err = normalizeFile(file)
		if err != nil {
			return Set{}, err
		}
		if err := s.saveMutation(file); err != nil {
			return Set{}, err
		}
		return updated, nil
	}
	return Set{}, fmt.Errorf("Skill Set not found")
}

// Delete removes one saved recipe without touching skills or Pending state.
func (s Store) Delete(id string) (Set, error) {
	file, err := s.Load()
	if err != nil {
		return Set{}, err
	}
	id = strings.TrimSpace(id)
	for index, existing := range file.Sets {
		if existing.ID != id {
			continue
		}
		file.Sets = append(file.Sets[:index], file.Sets[index+1:]...)
		if file.Sets == nil {
			file.Sets = []Set{}
		}
		if err := s.saveMutation(file); err != nil {
			return Set{}, err
		}
		return existing, nil
	}
	return Set{}, fmt.Errorf("Skill Set not found")
}

// Get returns a set by opaque ID.
func (f File) Get(id string) (Set, bool) {
	for _, set := range f.Sets {
		if set.ID == id {
			return set, true
		}
	}
	return Set{}, false
}

func (s Store) saveMutation(file File) error {
	file, err := normalizeFile(file)
	if err != nil {
		return err
	}
	if err := state.New(s.paths).Secure(); err != nil {
		return err
	}
	if err := s.backupExisting(); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.paths.StateDir, "skill-sets-*.json")
	if err != nil {
		return fmt.Errorf("create temporary Skill Sets file: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(privateFileMode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary Skill Sets file: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(file); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode Skill Sets: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary Skill Sets file: %w", err)
	}
	if err := os.Rename(temporaryPath, s.paths.SkillSetsFile); err != nil {
		return fmt.Errorf("replace Skill Sets %s: %w", s.paths.SkillSetsFile, err)
	}
	if err := os.Chmod(s.paths.SkillSetsFile, privateFileMode); err != nil {
		return fmt.Errorf("secure Skill Sets %s: %w", s.paths.SkillSetsFile, err)
	}
	removeTemporary = false
	return nil
}

func (s Store) backupExisting() error {
	source, err := os.Open(s.paths.SkillSetsFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open Skill Sets for backup: %w", err)
	}
	defer source.Close()
	if err := os.MkdirAll(s.paths.BackupDir, privateDirMode); err != nil {
		return fmt.Errorf("create backup directory %s: %w", s.paths.BackupDir, err)
	}
	if err := os.Chmod(s.paths.BackupDir, privateDirMode); err != nil {
		return fmt.Errorf("secure backup directory %s: %w", s.paths.BackupDir, err)
	}
	backupPath := filepath.Join(s.paths.BackupDir, "skill-sets-"+s.now().UTC().Format(backupTimeFormat)+".json")
	destination, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
	if err != nil {
		return fmt.Errorf("create Skill Sets backup %s: %w", backupPath, err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		return fmt.Errorf("copy Skill Sets backup: %w", err)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("close Skill Sets backup: %w", err)
	}
	return s.rotateBackups(backupPath)
}

func (s Store) rotateBackups(newest string) error {
	entries, err := os.ReadDir(s.paths.BackupDir)
	if err != nil {
		return fmt.Errorf("inspect Skill Sets backups: %w", err)
	}
	type backup struct {
		path string
		when time.Time
	}
	backups := []backup{}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasPrefix(name, "skill-sets-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		stamp := strings.TrimSuffix(strings.TrimPrefix(name, "skill-sets-"), ".json")
		when, parseErr := time.Parse(backupTimeFormat, stamp)
		if parseErr != nil {
			continue
		}
		backups = append(backups, backup{path: filepath.Join(s.paths.BackupDir, name), when: when})
	}
	sort.SliceStable(backups, func(i, j int) bool { return backups[i].when.After(backups[j].when) })
	cutoff := s.now().UTC().Add(-maxBackupAge)
	kept := 1
	for _, candidate := range backups {
		if candidate.path == newest {
			continue
		}
		if kept >= maxBackups || candidate.when.Before(cutoff) {
			if err := os.Remove(candidate.path); err != nil {
				return fmt.Errorf("remove expired Skill Sets backup %s: %w", candidate.path, err)
			}
			continue
		}
		kept++
	}
	return nil
}

func emptyFile() File {
	return File{Version: formatVersion, Sets: []Set{}}
}

func normalizeFile(file File) (File, error) {
	if file.Version != formatVersion {
		return File{}, fmt.Errorf("unsupported Skill Sets version %d", file.Version)
	}
	if file.Sets == nil {
		file.Sets = []Set{}
	}
	seenIDs := map[string]bool{}
	seenNames := map[string]bool{}
	for index := range file.Sets {
		normalized, err := normalizeSet(file.Sets[index])
		if err != nil {
			return File{}, err
		}
		nameKey := strings.ToLower(normalized.Name)
		if seenIDs[normalized.ID] {
			return File{}, fmt.Errorf("duplicate Skill Set id %q", normalized.ID)
		}
		if seenNames[nameKey] {
			return File{}, fmt.Errorf("duplicate Skill Set name %q", normalized.Name)
		}
		seenIDs[normalized.ID] = true
		seenNames[nameKey] = true
		file.Sets[index] = normalized
	}
	sort.SliceStable(file.Sets, func(i, j int) bool {
		left, right := strings.ToLower(file.Sets[i].Name), strings.ToLower(file.Sets[j].Name)
		if left != right {
			return left < right
		}
		return file.Sets[i].ID < file.Sets[j].ID
	})
	return file, nil
}

func normalizeSet(set Set) (Set, error) {
	set.ID = strings.TrimSpace(set.ID)
	set.Name = strings.TrimSpace(set.Name)
	set.Description = strings.TrimSpace(set.Description)
	if set.ID == "" {
		return Set{}, fmt.Errorf("Skill Set id is required")
	}
	if set.Name == "" {
		return Set{}, fmt.Errorf("Skill Set name is required")
	}
	if set.CreatedAt.IsZero() || set.UpdatedAt.IsZero() {
		return Set{}, fmt.Errorf("Skill Set timestamps are required")
	}
	seen := map[string]bool{}
	normalized := make([]string, 0, len(set.Skills))
	for _, skill := range set.Skills {
		skill = strings.TrimSpace(skill)
		if !validSkillName(skill) {
			return Set{}, fmt.Errorf("invalid Skill Set member %q", skill)
		}
		if seen[skill] {
			continue
		}
		seen[skill] = true
		normalized = append(normalized, skill)
	}
	if len(normalized) == 0 {
		return Set{}, fmt.Errorf("Skill Set must contain at least one skill")
	}
	sort.Strings(normalized)
	set.Skills = normalized
	set.CreatedAt = set.CreatedAt.UTC()
	set.UpdatedAt = set.UpdatedAt.UTC()
	return set, nil
}

func duplicateName(sets []Set, exceptID, name string) bool {
	name = strings.TrimSpace(name)
	for _, set := range sets {
		if set.ID != exceptID && strings.EqualFold(set.Name, name) {
			return true
		}
	}
	return false
}

func validSkillName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsRune(name, '\x00') && filepath.Base(name) == name
}

func secureExistingFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Skill Sets %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe Skill Sets path %s: expected a regular file", path)
	}
	if err := os.Chmod(path, privateFileMode); err != nil {
		return fmt.Errorf("secure Skill Sets %s: %w", path, err)
	}
	return nil
}

func randomID() (string, error) {
	buffer := make([]byte, skillSetIDByteCount)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate Skill Set id: %w", err)
	}
	return "set-" + hex.EncodeToString(buffer), nil
}
