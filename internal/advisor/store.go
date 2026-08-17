package advisor

import (
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

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
	"golang.org/x/sys/unix"
)

const (
	privateDirMode  = 0o700
	privateFileMode = 0o600
	backupPrefix    = "advisor-activations-"
	backupSuffix    = ".json"
	backupTime      = "20060102T150405.000000000Z"
	maxBackups      = 10
	maxBackupAge    = 30 * 24 * time.Hour
)

type store struct {
	paths paths.Paths
	now   func() time.Time
}

func newStore(p paths.Paths) store {
	return store{paths: p, now: time.Now}
}

func (s store) load() (file, error) {
	if err := secureRegularFile(s.paths.AdvisorFile); err != nil {
		return file{}, err
	}
	data, err := os.ReadFile(s.paths.AdvisorFile)
	if errors.Is(err, os.ErrNotExist) {
		return emptyFile(), nil
	}
	if err != nil {
		return file{}, fmt.Errorf("load advisor activations %s: %w", s.paths.AdvisorFile, err)
	}
	var decoded file
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return file{}, fmt.Errorf("decode advisor activations %s: %w", s.paths.AdvisorFile, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return file{}, fmt.Errorf("decode advisor activations %s: trailing JSON data", s.paths.AdvisorFile)
	}
	if decoded.Version != fileVersion {
		return file{}, fmt.Errorf("unsupported advisor activation version %d; this binary supports version %d", decoded.Version, fileVersion)
	}
	return validateFile(decoded, s.paths)
}

func (s store) save(contents file) error {
	validated, err := validateFile(contents, s.paths)
	if err != nil {
		return err
	}
	if err := state.New(s.paths).Secure(); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.paths.StateDir, "advisor-activations-*.json")
	if err != nil {
		return fmt.Errorf("create temporary advisor activation file: %w", err)
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
		return fmt.Errorf("secure temporary advisor activation file: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(validated); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode advisor activations: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary advisor activation file: %w", err)
	}
	if err := os.Rename(temporaryPath, s.paths.AdvisorFile); err != nil {
		return fmt.Errorf("replace advisor activations %s: %w", s.paths.AdvisorFile, err)
	}
	if err := os.Chmod(s.paths.AdvisorFile, privateFileMode); err != nil {
		return fmt.Errorf("secure advisor activations %s: %w", s.paths.AdvisorFile, err)
	}
	removeTemporary = false
	return nil
}

func (s store) backupExisting() error {
	if err := state.New(s.paths).Secure(); err != nil {
		return err
	}
	if err := secureRegularFile(s.paths.AdvisorFile); err != nil {
		return err
	}
	source, err := os.Open(s.paths.AdvisorFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open advisor activations for backup: %w", err)
	}
	defer source.Close()
	if err := os.MkdirAll(s.paths.BackupDir, privateDirMode); err != nil {
		return fmt.Errorf("create backup directory %s: %w", s.paths.BackupDir, err)
	}
	if err := os.Chmod(s.paths.BackupDir, privateDirMode); err != nil {
		return fmt.Errorf("secure backup directory %s: %w", s.paths.BackupDir, err)
	}
	backupPath, destination, err := s.createBackupFile()
	if err != nil {
		return err
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		return fmt.Errorf("copy advisor activation backup: %w", err)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("close advisor activation backup: %w", err)
	}
	return s.rotateBackups(backupPath)
}

func (s store) createBackupFile() (string, *os.File, error) {
	when := s.now().UTC()
	for attempt := 0; attempt < 100; attempt++ {
		backupPath := filepath.Join(s.paths.BackupDir, backupPrefix+when.Add(time.Duration(attempt)).Format(backupTime)+backupSuffix)
		destination, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, fmt.Errorf("create advisor activation backup %s: %w", backupPath, err)
		}
		return backupPath, destination, nil
	}
	return "", nil, fmt.Errorf("create a unique advisor activation backup")
}

func (s store) rotateBackups(newestPath string) error {
	entries, err := os.ReadDir(s.paths.BackupDir)
	if err != nil {
		return fmt.Errorf("inspect advisor activation backups: %w", err)
	}
	type candidate struct {
		path string
		when time.Time
	}
	backups := []candidate{}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasPrefix(entry.Name(), backupPrefix) || !strings.HasSuffix(entry.Name(), backupSuffix) {
			continue
		}
		value := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), backupPrefix), backupSuffix)
		when, err := time.Parse(backupTime, value)
		if err != nil {
			continue
		}
		path := filepath.Join(s.paths.BackupDir, entry.Name())
		if err := secureRegularFile(path); err != nil {
			return err
		}
		backups = append(backups, candidate{path: path, when: when})
	}
	sort.SliceStable(backups, func(i, j int) bool { return backups[i].when.After(backups[j].when) })
	cutoff := s.now().UTC().Add(-maxBackupAge)
	kept := 1
	for _, backup := range backups {
		if backup.path == newestPath {
			continue
		}
		if kept >= maxBackups || backup.when.Before(cutoff) {
			if err := os.Remove(backup.path); err != nil {
				return fmt.Errorf("remove expired advisor activation backup %s: %w", backup.path, err)
			}
			continue
		}
		kept++
	}
	return nil
}

type fileLock struct {
	file *os.File
}

func (s store) lock(exclusive bool) (*fileLock, error) {
	if err := state.New(s.paths).Secure(); err != nil {
		return nil, err
	}
	fd, err := unix.Open(s.paths.AdvisorLockFile, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, privateFileMode)
	if err != nil {
		return nil, fmt.Errorf("open advisor lock %s: %w", s.paths.AdvisorLockFile, err)
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("inspect advisor lock %s: %w", s.paths.AdvisorLockFile, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("unsafe advisor lock %s: expected a regular file", s.paths.AdvisorLockFile)
	}
	if err := unix.Fchmod(fd, privateFileMode); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("secure advisor lock %s: %w", s.paths.AdvisorLockFile, err)
	}
	operation := unix.LOCK_SH
	if exclusive {
		operation = unix.LOCK_EX
	}
	if err := unix.Flock(fd, operation); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("lock advisor activations: %w", err)
	}
	return &fileLock{file: os.NewFile(uintptr(fd), s.paths.AdvisorLockFile)}, nil
}

func (l *fileLock) close() error {
	if l == nil || l.file == nil {
		return nil
	}
	if err := unix.Flock(int(l.file.Fd()), unix.LOCK_UN); err != nil {
		_ = l.file.Close()
		return fmt.Errorf("unlock advisor activations: %w", err)
	}
	return l.file.Close()
}

func validateFile(contents file, p paths.Paths) (file, error) {
	if contents.Version != fileVersion {
		return file{}, fmt.Errorf("unsupported advisor activation version %d; this binary supports version %d", contents.Version, fileVersion)
	}
	contents.normalizeOrder()
	receipts := make(map[string]receipt, len(contents.Receipts))
	for _, current := range contents.Receipts {
		if !validReceiptID(current.ID) {
			return file{}, fmt.Errorf("invalid advisor receipt ID %q", current.ID)
		}
		if _, exists := receipts[current.ID]; exists {
			return file{}, fmt.Errorf("duplicate advisor receipt ID %q", current.ID)
		}
		if _, ok := model.ParseTool(current.Tool.String()); !ok {
			return file{}, fmt.Errorf("invalid advisor receipt tool %q", current.Tool)
		}
		if current.CreatedAt.IsZero() {
			return file{}, fmt.Errorf("advisor receipt %s has no creation time", current.ID)
		}
		if len(current.Skills) == 0 || len(current.Skills) > MaxSkillsPerActivation {
			return file{}, fmt.Errorf("advisor receipt %s has %d skills; expected 1-%d", current.ID, len(current.Skills), MaxSkillsPerActivation)
		}
		if err := validateUniqueSkillNames(current.Skills); err != nil {
			return file{}, fmt.Errorf("advisor receipt %s: %w", current.ID, err)
		}
		receipts[current.ID] = current
	}
	leaseKeys := map[string]lease{}
	for _, current := range contents.Leases {
		if _, ok := model.ParseTool(current.Tool.String()); !ok {
			return file{}, fmt.Errorf("invalid advisor lease tool %q", current.Tool)
		}
		if !validSkillName(current.SkillName) {
			return file{}, fmt.Errorf("invalid advisor lease skill name %q", current.SkillName)
		}
		key := current.Tool.String() + "\x00" + current.SkillName
		if _, exists := leaseKeys[key]; exists {
			return file{}, fmt.Errorf("duplicate advisor lease for %s/%s", current.Tool, current.SkillName)
		}
		if err := validateLeasePaths(current, p); err != nil {
			return file{}, err
		}
		if len(current.ReceiptIDs) == 0 {
			return file{}, fmt.Errorf("advisor lease %s/%s has no receipts", current.Tool, current.SkillName)
		}
		seenClaims := map[string]bool{}
		for _, receiptID := range current.ReceiptIDs {
			if seenClaims[receiptID] {
				return file{}, fmt.Errorf("advisor lease %s/%s repeats receipt %s", current.Tool, current.SkillName, receiptID)
			}
			seenClaims[receiptID] = true
			owner, ok := receipts[receiptID]
			if !ok || owner.Tool != current.Tool || !containsString(owner.Skills, current.SkillName) {
				return file{}, fmt.Errorf("advisor lease %s/%s references inconsistent receipt %s", current.Tool, current.SkillName, receiptID)
			}
		}
		leaseKeys[key] = current
	}
	for _, current := range contents.Receipts {
		for _, skillName := range current.Skills {
			key := current.Tool.String() + "\x00" + skillName
			owned, ok := leaseKeys[key]
			if !ok || !containsString(owned.ReceiptIDs, current.ID) {
				return file{}, fmt.Errorf("advisor receipt %s has inconsistent claim for %s/%s", current.ID, current.Tool, skillName)
			}
		}
	}
	return contents, nil
}

func validateLeasePaths(current lease, p paths.Paths) error {
	activeDir, ok := p.UserSkillsDirFor(current.Tool)
	if !ok {
		return fmt.Errorf("unsupported advisor lease tool %q", current.Tool)
	}
	disabledDir, _ := p.DisabledDirFor(current.Tool)
	if filepath.Clean(current.OriginalPath) != filepath.Join(activeDir, current.SkillName) || filepath.Clean(current.DisabledPath) != filepath.Join(disabledDir, current.SkillName) {
		return fmt.Errorf("advisor lease %s/%s contains unexpected paths", current.Tool, current.SkillName)
	}
	if current.EntryType != model.EntryTypeDir && current.EntryType != model.EntryTypeSymlink {
		return fmt.Errorf("advisor lease %s/%s has invalid entry type %q", current.Tool, current.SkillName, current.EntryType)
	}
	if current.EntryType == model.EntryTypeSymlink && current.SymlinkTarget == "" {
		return fmt.Errorf("advisor lease %s/%s has no symlink target", current.Tool, current.SkillName)
	}
	if current.EntryType == model.EntryTypeDir && current.SymlinkTarget != "" {
		return fmt.Errorf("advisor lease %s/%s has a directory symlink target", current.Tool, current.SkillName)
	}
	return nil
}

func validateUniqueSkillNames(names []string) error {
	seen := map[string]bool{}
	for _, name := range names {
		if !validSkillName(name) {
			return fmt.Errorf("invalid skill name %q", name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate skill name %q", name)
		}
		seen[name] = true
	}
	return nil
}

func validSkillName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name && strings.TrimSpace(name) == name
}

func validReceiptID(id string) bool {
	if len(id) != 32 || strings.ToLower(id) != id {
		return false
	}
	decoded, err := hex.DecodeString(id)
	return err == nil && len(decoded) == 16
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func secureRegularFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect private advisor file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe private advisor path %s: expected a regular file", path)
	}
	if err := os.Chmod(path, privateFileMode); err != nil {
		return fmt.Errorf("secure private advisor file %s: %w", path, err)
	}
	return nil
}
