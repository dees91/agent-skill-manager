package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

const manifestVersion = 2

// Store persists Skill Manager state under ~/.skill-manager.
type Store struct {
	paths paths.Paths
	now   func() time.Time
}

// New creates a state store for the provided paths.
func New(p paths.Paths) Store {
	return Store{
		paths: p,
		now:   time.Now,
	}
}

// Manifest is the on-disk state.json shape.
type Manifest struct {
	Version      int                `json:"version"`
	Disabled     []DisabledEntry    `json:"disabled"`
	Repositories []RepositoryEntry  `json:"repositories"`
	LocalSources []LocalSourceEntry `json:"localSources"`
}

// DisabledEntry records enough information to restore a disabled skill entry.
type DisabledEntry struct {
	Tool          model.Tool        `json:"tool"`
	SkillName     string            `json:"skillName"`
	OriginalPath  string            `json:"originalPath"`
	DisabledPath  string            `json:"disabledPath"`
	EntryType     model.EntryType   `json:"entryType"`
	SymlinkTarget string            `json:"symlinkTarget,omitempty"`
	Source        model.SourceLabel `json:"source"`
	Group         model.GroupLabel  `json:"group,omitempty"`
	DisabledAt    time.Time         `json:"disabledAt"`
}

// RepositoryEntry records a Skill Manager managed repository checkout.
type RepositoryEntry struct {
	OriginalURL     string                `json:"originalUrl"`
	CanonicalURL    string                `json:"canonicalUrl,omitempty"`
	Host            string                `json:"host"`
	RepoPath        string                `json:"repoPath"`
	CheckoutPath    string                `json:"checkoutPath"`
	Group           model.GroupLabel      `json:"group"`
	InstalledSkills []InstalledSkillEntry `json:"installedSkills"`
	InstalledAt     time.Time             `json:"installedAt"`
	LastSeenCommit  string                `json:"lastSeenCommit,omitempty"`
}

// InstalledSkillEntry records one skill installed from a managed source.
type InstalledSkillEntry struct {
	Name         string       `json:"name"`
	RelativePath string       `json:"relativePath"`
	Tools        []model.Tool `json:"tools"`
}

// LocalSourceEntry records skills linked directly from a user-owned local directory.
type LocalSourceEntry struct {
	OriginalPath    string                `json:"originalPath"`
	CanonicalPath   string                `json:"canonicalPath"`
	Group           model.GroupLabel      `json:"group"`
	InstalledSkills []InstalledSkillEntry `json:"installedSkills"`
	InstalledAt     time.Time             `json:"installedAt"`
}

// EntryKey identifies one disabled skill for one tool.
type EntryKey struct {
	Tool      model.Tool
	SkillName string
}

// Load reads state.json. A missing file returns an empty manifest.
func (s Store) Load() (Manifest, error) {
	data, err := os.ReadFile(s.paths.StateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return emptyManifest(), nil
		}
		return Manifest{}, fmt.Errorf("load state manifest %s: %w", s.paths.StateFile, err)
	}

	manifest := emptyManifest()
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode state manifest %s: %w", s.paths.StateFile, err)
	}
	if manifest.Version < 0 || manifest.Version > manifestVersion {
		return Manifest{}, fmt.Errorf("unsupported state manifest version %d; this binary supports up to version %d", manifest.Version, manifestVersion)
	}
	return normalizeManifest(manifest), nil
}

// Save writes state.json as valid JSON using a same-directory temp file rename.
func (s Store) Save(manifest Manifest) error {
	if manifest.Version < 0 || manifest.Version > manifestVersion {
		return fmt.Errorf("unsupported state manifest version %d; this binary supports up to version %d", manifest.Version, manifestVersion)
	}
	manifest = normalizeManifest(manifest)
	if err := os.MkdirAll(s.paths.StateDir, 0o755); err != nil {
		return fmt.Errorf("create state directory %s: %w", s.paths.StateDir, err)
	}

	tmp, err := os.CreateTemp(s.paths.StateDir, "state-*.json")
	if err != nil {
		return fmt.Errorf("create temporary state manifest: %w", err)
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode state manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary state manifest: %w", err)
	}
	if err := os.Rename(tmpPath, s.paths.StateFile); err != nil {
		return fmt.Errorf("replace state manifest %s: %w", s.paths.StateFile, err)
	}
	removeTemp = false
	return nil
}

// BackupExisting copies the current state manifest into backups if it exists.
func (s Store) BackupExisting() (string, error) {
	source, err := os.Open(s.paths.StateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("open state manifest for backup: %w", err)
	}
	defer source.Close()

	if err := os.MkdirAll(s.paths.BackupDir, 0o755); err != nil {
		return "", fmt.Errorf("create backup directory %s: %w", s.paths.BackupDir, err)
	}

	backupPath := filepath.Join(s.paths.BackupDir, "state-"+s.now().UTC().Format("20060102T150405.000000000Z")+".json")
	destination, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create state backup %s: %w", backupPath, err)
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return "", fmt.Errorf("copy state backup %s: %w", backupPath, err)
	}
	if err := destination.Close(); err != nil {
		return "", fmt.Errorf("close state backup %s: %w", backupPath, err)
	}

	return backupPath, nil
}

// DisabledPath returns the disabled layout path for a tool-specific skill.
func (s Store) DisabledPath(tool model.Tool, skillName string) (string, error) {
	baseDir, ok := s.paths.DisabledDirFor(tool)
	if !ok {
		return "", fmt.Errorf("unsupported tool %q", tool)
	}
	if !validSkillName(skillName) {
		return "", fmt.Errorf("invalid skill name %q", skillName)
	}
	return filepath.Join(baseDir, skillName), nil
}

// Get returns a disabled entry by tool and skill name.
func (m Manifest) Get(tool model.Tool, skillName string) (DisabledEntry, bool) {
	for _, entry := range m.Disabled {
		if entry.Tool == tool && entry.SkillName == skillName {
			return entry, true
		}
	}
	return DisabledEntry{}, false
}

// Upsert adds or replaces a disabled entry by tool and skill name.
func (m *Manifest) Upsert(entry DisabledEntry) {
	m.ensure()
	for i := range m.Disabled {
		if m.Disabled[i].Tool == entry.Tool && m.Disabled[i].SkillName == entry.SkillName {
			m.Disabled[i] = entry
			return
		}
	}
	m.Disabled = append(m.Disabled, entry)
}

// Remove deletes a disabled entry by tool and skill name.
func (m *Manifest) Remove(tool model.Tool, skillName string) bool {
	for i := range m.Disabled {
		if m.Disabled[i].Tool == tool && m.Disabled[i].SkillName == skillName {
			m.Disabled = append(m.Disabled[:i], m.Disabled[i+1:]...)
			if m.Disabled == nil {
				m.Disabled = []DisabledEntry{}
			}
			return true
		}
	}
	return false
}

// GetRepository returns a managed repository entry by normalized host and repo path.
func (m Manifest) GetRepository(host, repoPath string) (RepositoryEntry, bool) {
	for _, entry := range m.Repositories {
		if entry.Host == host && entry.RepoPath == repoPath {
			return entry, true
		}
	}
	return RepositoryEntry{}, false
}

// UpsertRepository adds or replaces a managed repository entry by host and repo path.
func (m *Manifest) UpsertRepository(entry RepositoryEntry) {
	m.ensure()
	entry = normalizeRepositoryEntry(entry)
	for i := range m.Repositories {
		if m.Repositories[i].Host == entry.Host && m.Repositories[i].RepoPath == entry.RepoPath {
			m.Repositories[i] = entry
			m.Repositories = normalizeRepositories(m.Repositories)
			return
		}
	}
	m.Repositories = append(m.Repositories, entry)
	m.Repositories = normalizeRepositories(m.Repositories)
}

// RemoveRepository deletes a managed repository by normalized host and repo path.
func (m *Manifest) RemoveRepository(host, repoPath string) bool {
	for i := range m.Repositories {
		if m.Repositories[i].Host == host && m.Repositories[i].RepoPath == repoPath {
			m.Repositories = append(m.Repositories[:i], m.Repositories[i+1:]...)
			if m.Repositories == nil {
				m.Repositories = []RepositoryEntry{}
			}
			return true
		}
	}
	return false
}

// GetLocalSource returns a local source by canonical absolute path.
func (m Manifest) GetLocalSource(canonicalPath string) (LocalSourceEntry, bool) {
	canonicalPath = filepath.Clean(canonicalPath)
	for _, entry := range m.LocalSources {
		if filepath.Clean(entry.CanonicalPath) == canonicalPath {
			return entry, true
		}
	}
	return LocalSourceEntry{}, false
}

// UpsertLocalSource adds or replaces a local source by canonical path.
func (m *Manifest) UpsertLocalSource(entry LocalSourceEntry) {
	m.ensure()
	entry = normalizeLocalSourceEntry(entry)
	for i := range m.LocalSources {
		if filepath.Clean(m.LocalSources[i].CanonicalPath) == entry.CanonicalPath {
			m.LocalSources[i] = entry
			m.LocalSources = normalizeLocalSources(m.LocalSources)
			return
		}
	}
	m.LocalSources = append(m.LocalSources, entry)
	m.LocalSources = normalizeLocalSources(m.LocalSources)
}

// RemoveLocalSource deletes a local source by canonical path.
func (m *Manifest) RemoveLocalSource(canonicalPath string) bool {
	canonicalPath = filepath.Clean(canonicalPath)
	for i := range m.LocalSources {
		if filepath.Clean(m.LocalSources[i].CanonicalPath) == canonicalPath {
			m.LocalSources = append(m.LocalSources[:i], m.LocalSources[i+1:]...)
			if m.LocalSources == nil {
				m.LocalSources = []LocalSourceEntry{}
			}
			return true
		}
	}
	return false
}

func (m *Manifest) ensure() {
	if m.Version == 0 || m.Version == 1 {
		m.Version = manifestVersion
	}
	if m.Disabled == nil {
		m.Disabled = []DisabledEntry{}
	}
	if m.Repositories == nil {
		m.Repositories = []RepositoryEntry{}
	}
	if m.LocalSources == nil {
		m.LocalSources = []LocalSourceEntry{}
	}
}

func emptyManifest() Manifest {
	return Manifest{
		Version:      manifestVersion,
		Disabled:     []DisabledEntry{},
		Repositories: []RepositoryEntry{},
		LocalSources: []LocalSourceEntry{},
	}
}

func normalizeManifest(manifest Manifest) Manifest {
	manifest.ensure()
	manifest.Repositories = normalizeRepositories(manifest.Repositories)
	manifest.LocalSources = normalizeLocalSources(manifest.LocalSources)
	return manifest
}

func normalizeRepositories(repositories []RepositoryEntry) []RepositoryEntry {
	if repositories == nil {
		return []RepositoryEntry{}
	}
	byIdentity := map[string]RepositoryEntry{}
	for _, entry := range repositories {
		entry = normalizeRepositoryEntry(entry)
		byIdentity[repositoryKey(entry.Host, entry.RepoPath)] = entry
	}
	normalized := make([]RepositoryEntry, 0, len(byIdentity))
	for _, entry := range byIdentity {
		normalized = append(normalized, entry)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].Host != normalized[j].Host {
			return normalized[i].Host < normalized[j].Host
		}
		return normalized[i].RepoPath < normalized[j].RepoPath
	})
	return normalized
}

func repositoryKey(host, repoPath string) string {
	return host + "\x00" + repoPath
}

func normalizeRepositoryEntry(entry RepositoryEntry) RepositoryEntry {
	entry.InstalledSkills = normalizeInstalledSkills(entry.InstalledSkills)
	return entry
}

func normalizeLocalSources(sources []LocalSourceEntry) []LocalSourceEntry {
	if sources == nil {
		return []LocalSourceEntry{}
	}
	byPath := map[string]LocalSourceEntry{}
	for _, entry := range sources {
		entry = normalizeLocalSourceEntry(entry)
		byPath[entry.CanonicalPath] = entry
	}
	normalized := make([]LocalSourceEntry, 0, len(byPath))
	for _, entry := range byPath {
		normalized = append(normalized, entry)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].CanonicalPath < normalized[j].CanonicalPath
	})
	return normalized
}

func normalizeLocalSourceEntry(entry LocalSourceEntry) LocalSourceEntry {
	if entry.OriginalPath != "" {
		entry.OriginalPath = filepath.Clean(entry.OriginalPath)
	}
	if entry.CanonicalPath != "" {
		entry.CanonicalPath = filepath.Clean(entry.CanonicalPath)
	}
	entry.InstalledSkills = normalizeInstalledSkills(entry.InstalledSkills)
	return entry
}

func normalizeInstalledSkills(skills []InstalledSkillEntry) []InstalledSkillEntry {
	if skills == nil {
		return []InstalledSkillEntry{}
	}
	for i := range skills {
		skills[i].Tools = normalizeTools(skills[i].Tools)
	}
	sort.SliceStable(skills, func(i, j int) bool {
		if skills[i].Name != skills[j].Name {
			return skills[i].Name < skills[j].Name
		}
		return skills[i].RelativePath < skills[j].RelativePath
	})
	return skills
}

func normalizeTools(tools []model.Tool) []model.Tool {
	if tools == nil {
		return []model.Tool{}
	}
	seen := map[model.Tool]bool{}
	normalized := make([]model.Tool, 0, len(tools))
	for _, tool := range tools {
		if seen[tool] {
			continue
		}
		seen[tool] = true
		normalized = append(normalized, tool)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return toolOrder(normalized[i]) < toolOrder(normalized[j])
	})
	return normalized
}

func toolOrder(tool model.Tool) string {
	for i, known := range model.Tools() {
		if tool == known {
			return fmt.Sprintf("%03d:", i)
		}
	}
	return "999:" + tool.String()
}

func validSkillName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name
}
