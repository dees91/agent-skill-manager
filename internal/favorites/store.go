// Package favorites persists tool-agnostic skill bookmarks without coupling
// them to the ownership and toggle manifest.
package favorites

import (
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
	formatVersion    = 1
	privateDirMode   = 0o700
	privateFileMode  = 0o600
	maxBackups       = 10
	maxBackupAge     = 30 * 24 * time.Hour
	backupTimeFormat = "20060102T150405.000000000Z"
)

// File is the versioned on-disk favorites.json shape.
type File struct {
	Version int      `json:"version"`
	Skills  []string `json:"skills"`
}

// Contains reports whether a skill basename is saved as a favorite.
func (f File) Contains(name string) bool {
	index := sort.SearchStrings(f.Skills, name)
	return index < len(f.Skills) && f.Skills[index] == name
}

// Store owns atomic persistence and bounded backups for favorites.
type Store struct {
	paths paths.Paths
	now   func() time.Time
}

// New creates a store rooted in the provided synthetic or real home.
func New(p paths.Paths) Store {
	return Store{paths: p, now: time.Now}
}

// Load reads and validates favorites. A missing file is an empty collection.
func (s Store) Load() (File, error) {
	if err := secureExistingFile(s.paths.FavoritesFile); err != nil {
		return File{}, err
	}
	data, err := os.ReadFile(s.paths.FavoritesFile)
	if errors.Is(err, os.ErrNotExist) {
		return emptyFile(), nil
	}
	if err != nil {
		return File{}, fmt.Errorf("load favorites %s: %w", s.paths.FavoritesFile, err)
	}
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("decode favorites %s: %w", s.paths.FavoritesFile, err)
	}
	if file.Version != formatVersion {
		return File{}, fmt.Errorf("unsupported favorites version %d; this binary supports version %d", file.Version, formatVersion)
	}
	normalized, err := normalizeFile(file)
	if err != nil {
		return File{}, fmt.Errorf("validate favorites %s: %w", s.paths.FavoritesFile, err)
	}
	return normalized, nil
}

// Set idempotently adds or removes one validated skill basename.
func (s Store) Set(name string, favorite bool) (File, error) {
	name = strings.TrimSpace(name)
	if !validSkillName(name) {
		return File{}, fmt.Errorf("invalid favorite skill %q", name)
	}
	file, err := s.Load()
	if err != nil {
		return File{}, err
	}
	present := file.Contains(name)
	if present == favorite {
		return file, nil
	}
	if favorite {
		file.Skills = append(file.Skills, name)
	} else {
		index := sort.SearchStrings(file.Skills, name)
		file.Skills = append(file.Skills[:index], file.Skills[index+1:]...)
	}
	file, err = normalizeFile(file)
	if err != nil {
		return File{}, err
	}
	if err := s.saveMutation(file); err != nil {
		return File{}, err
	}
	return file, nil
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
	temporary, err := os.CreateTemp(s.paths.StateDir, "favorites-*.json")
	if err != nil {
		return fmt.Errorf("create temporary favorites file: %w", err)
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
		return fmt.Errorf("secure temporary favorites file: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(file); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode favorites: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary favorites file: %w", err)
	}
	if err := os.Rename(temporaryPath, s.paths.FavoritesFile); err != nil {
		return fmt.Errorf("replace favorites %s: %w", s.paths.FavoritesFile, err)
	}
	if err := os.Chmod(s.paths.FavoritesFile, privateFileMode); err != nil {
		return fmt.Errorf("secure favorites %s: %w", s.paths.FavoritesFile, err)
	}
	removeTemporary = false
	return nil
}

func (s Store) backupExisting() error {
	source, err := os.Open(s.paths.FavoritesFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open favorites for backup: %w", err)
	}
	defer source.Close()
	if err := os.MkdirAll(s.paths.BackupDir, privateDirMode); err != nil {
		return fmt.Errorf("create backup directory %s: %w", s.paths.BackupDir, err)
	}
	if err := os.Chmod(s.paths.BackupDir, privateDirMode); err != nil {
		return fmt.Errorf("secure backup directory %s: %w", s.paths.BackupDir, err)
	}
	backupPath := filepath.Join(s.paths.BackupDir, "favorites-"+s.now().UTC().Format(backupTimeFormat)+".json")
	destination, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
	if err != nil {
		return fmt.Errorf("create favorites backup %s: %w", backupPath, err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		return fmt.Errorf("copy favorites backup: %w", err)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("close favorites backup: %w", err)
	}
	return s.rotateBackups(backupPath)
}

func (s Store) rotateBackups(newest string) error {
	entries, err := os.ReadDir(s.paths.BackupDir)
	if err != nil {
		return fmt.Errorf("inspect favorites backups: %w", err)
	}
	type backup struct {
		path string
		when time.Time
	}
	backups := []backup{}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasPrefix(name, "favorites-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		stamp := strings.TrimSuffix(strings.TrimPrefix(name, "favorites-"), ".json")
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
				return fmt.Errorf("remove expired favorites backup %s: %w", candidate.path, err)
			}
			continue
		}
		kept++
	}
	return nil
}

func emptyFile() File {
	return File{Version: formatVersion, Skills: []string{}}
}

func normalizeFile(file File) (File, error) {
	if file.Version != formatVersion {
		return File{}, fmt.Errorf("unsupported favorites version %d", file.Version)
	}
	seen := map[string]bool{}
	normalized := make([]string, 0, len(file.Skills))
	for _, name := range file.Skills {
		name = strings.TrimSpace(name)
		if !validSkillName(name) {
			return File{}, fmt.Errorf("invalid favorite skill %q", name)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)
	file.Skills = normalized
	return file, nil
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
		return fmt.Errorf("inspect favorites %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe favorites path %s: expected a regular file", path)
	}
	if err := os.Chmod(path, privateFileMode); err != nil {
		return fmt.Errorf("secure favorites %s: %w", path, err)
	}
	return nil
}
