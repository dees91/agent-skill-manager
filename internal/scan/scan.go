package scan

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/metadata"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// Scanner reads skill entries from the configured filesystem paths.
type Scanner struct {
	paths paths.Paths
}

type scanContext struct {
	reposByRoot map[string]gitRepoMetadata
	localCells  map[string]localCellMetadata
}

type gitRepoMetadata struct {
	origin string
	commit string
	group  model.GroupLabel
}

type localCellMetadata struct {
	target    string
	group     model.GroupLabel
	ambiguous bool
}

// New creates a scanner for the provided paths.
func New(p paths.Paths) Scanner {
	return Scanner{paths: p}
}

// Managed scans toggleable Claude and Codex user skill directories.
func (s Scanner) Managed() ([]model.ToolSkill, error) {
	var skills []model.ToolSkill
	manifest, err := state.New(s.paths).Load()
	if err != nil {
		return nil, err
	}
	context := newScanContextWithManifest(manifest)
	skillsCLINames := metadata.ReadSkillsLockNames(s.paths.AgentsSkillLock)
	for _, tool := range model.Tools() {
		dir, ok := s.paths.UserSkillsDirFor(tool)
		if !ok {
			continue
		}

		toolSkills, err := s.scanManagedTool(context, tool, dir, skillsCLINames)
		if err != nil {
			return nil, err
		}
		skills = append(skills, toolSkills...)
	}

	return skills, nil
}

// ReadOnly scans non-toggleable Codex system and Claude plugin cache skills.
func (s Scanner) ReadOnly() ([]model.ToolSkill, error) {
	var skills []model.ToolSkill

	codexSystem, err := s.scanCodexSystem()
	if err != nil {
		return nil, err
	}
	skills = append(skills, codexSystem...)

	claudePlugins, err := s.scanClaudePlugins()
	if err != nil {
		return nil, err
	}
	skills = append(skills, claudePlugins...)

	return skills, nil
}

// All scans both managed and read-only skill sources.
func (s Scanner) All() ([]model.ToolSkill, error) {
	managed, err := s.Managed()
	if err != nil {
		return nil, err
	}

	disabled, err := s.Disabled()
	if err != nil {
		return nil, err
	}

	readOnly, err := s.ReadOnly()
	if err != nil {
		return nil, err
	}

	skills := append(managed, disabled...)
	return append(skills, readOnly...), nil
}

// Disabled scans state manifest entries for skills currently moved out of active dirs.
func (s Scanner) Disabled() ([]model.ToolSkill, error) {
	manifest, err := state.New(s.paths).Load()
	if err != nil {
		return nil, err
	}

	context := newScanContext()
	skills := make([]model.ToolSkill, 0, len(manifest.Disabled))
	for _, entry := range manifest.Disabled {
		if !isKnownTool(entry.Tool) {
			continue
		}
		skill, err := disabledEntryToSkill(context, entry)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}

	sort.SliceStable(skills, func(i, j int) bool {
		if skills[i].Tool != skills[j].Tool {
			return skills[i].Tool.String() < skills[j].Tool.String()
		}
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

func newScanContext() *scanContext {
	return &scanContext{reposByRoot: map[string]gitRepoMetadata{}, localCells: map[string]localCellMetadata{}}
}

func newScanContextWithManifest(manifest state.Manifest) *scanContext {
	context := newScanContext()
	for _, source := range manifest.LocalSources {
		for _, skill := range source.InstalledSkills {
			target := filepath.Clean(filepath.Join(source.CanonicalPath, filepath.FromSlash(skill.RelativePath)))
			for _, tool := range skill.Tools {
				key := tool.String() + "\x00" + skill.Name
				if existing, ok := context.localCells[key]; ok && (!sameFilesystemPath(existing.target, target) || existing.group != source.Group) {
					existing.ambiguous = true
					context.localCells[key] = existing
					continue
				}
				context.localCells[key] = localCellMetadata{target: target, group: source.Group}
			}
		}
	}
	return context
}

func (s Scanner) scanManagedTool(context *scanContext, tool model.Tool, baseDir string, skillsCLINames map[string]struct{}) ([]model.ToolSkill, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan %s skills in %s: %w", tool, baseDir, err)
	}

	skills := make([]model.ToolSkill, 0, len(entries))
	for _, entry := range entries {
		skill, ok, err := inspectManagedEntry(context, tool, baseDir, entry.Name(), skillsCLINames)
		if err != nil {
			return nil, err
		}
		if ok {
			skills = append(skills, skill)
		}
	}

	sort.SliceStable(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return skills, nil
}

func (s Scanner) scanCodexSystem() ([]model.ToolSkill, error) {
	entries, err := os.ReadDir(s.paths.CodexSystemSkills)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan Codex system skills in %s: %w", s.paths.CodexSystemSkills, err)
	}

	skills := make([]model.ToolSkill, 0, len(entries))
	for _, entry := range entries {
		skill, ok, err := inspectReadOnlyEntry(model.ToolCodex, s.paths.CodexSystemSkills, entry.Name(), model.SourceCodexSystem)
		if err != nil {
			return nil, err
		}
		if ok {
			skills = append(skills, skill)
		}
	}

	sort.SliceStable(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

func (s Scanner) scanClaudePlugins() ([]model.ToolSkill, error) {
	if _, err := os.Stat(s.paths.ClaudePluginCache); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan Claude plugin skills in %s: %w", s.paths.ClaudePluginCache, err)
	}

	var skills []model.ToolSkill
	err := filepath.WalkDir(s.paths.ClaudePluginCache, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("%s: %w", path, walkErr)
		}
		if entry.IsDir() || entry.Name() != "SKILL.md" {
			return nil
		}

		skillDir := filepath.Dir(path)
		if filepath.Base(filepath.Dir(skillDir)) != "skills" {
			return nil
		}

		name := filepath.Base(skillDir)
		skill, ok, err := inspectReadOnlyEntry(model.ToolClaude, filepath.Dir(skillDir), name, model.SourceClaudePlugin)
		if err != nil {
			return err
		}
		if ok {
			skills = append(skills, skill)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan Claude plugin skills in %s: %w", s.paths.ClaudePluginCache, err)
	}

	sort.SliceStable(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

func inspectManagedEntry(context *scanContext, tool model.Tool, baseDir, name string, skillsCLINames map[string]struct{}) (model.ToolSkill, bool, error) {
	entryPath := filepath.Join(baseDir, name)
	info, err := os.Lstat(entryPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ToolSkill{}, false, nil
		}
		return model.ToolSkill{}, false, fmt.Errorf("inspect %s skill %s: %w", tool, entryPath, err)
	}

	entryType := model.EntryTypeUnknown
	symlinkTarget := ""
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		entryType = model.EntryTypeSymlink
		target, err := os.Readlink(entryPath)
		if err != nil {
			return model.ToolSkill{}, false, fmt.Errorf("read symlink target %s: %w", entryPath, err)
		}
		symlinkTarget = target
	case info.IsDir():
		entryType = model.EntryTypeDir
	default:
		return model.ToolSkill{}, false, nil
	}

	skillFile := filepath.Join(entryPath, "SKILL.md")
	if !isRegularSkillFile(skillFile) {
		return model.ToolSkill{}, false, nil
	}

	skillMetadata := metadata.ReadSkillMetadata(skillFile, name)
	source, group, repoOrigin, repoCommit := classifyManagedSource(context, tool, name, entryPath, entryType, skillsCLINames)

	return model.ToolSkill{
		Tool:          tool,
		Name:          name,
		DisplayName:   skillMetadata.Name,
		Description:   skillMetadata.Description,
		State:         model.SkillStateOn,
		Source:        source,
		Group:         group,
		EntryType:     entryType,
		ActivePath:    entryPath,
		SkillFilePath: skillFile,
		SymlinkTarget: symlinkTarget,
		RepoOrigin:    repoOrigin,
		RepoCommit:    repoCommit,
		ReadOnly:      false,
	}, true, nil
}

func inspectReadOnlyEntry(tool model.Tool, baseDir, name string, source model.SourceLabel) (model.ToolSkill, bool, error) {
	entryPath := filepath.Join(baseDir, name)
	info, err := os.Lstat(entryPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.ToolSkill{}, false, nil
		}
		return model.ToolSkill{}, false, fmt.Errorf("inspect read-only skill %s: %w", entryPath, err)
	}

	entryType := model.EntryTypeUnknown
	symlinkTarget := ""
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		entryType = model.EntryTypeSymlink
		target, err := os.Readlink(entryPath)
		if err != nil {
			return model.ToolSkill{}, false, fmt.Errorf("read symlink target %s: %w", entryPath, err)
		}
		symlinkTarget = target
	case info.IsDir():
		entryType = model.EntryTypeDir
	default:
		return model.ToolSkill{}, false, nil
	}

	skillFile := filepath.Join(entryPath, "SKILL.md")
	if !isRegularSkillFile(skillFile) {
		return model.ToolSkill{}, false, nil
	}

	skillMetadata := metadata.ReadSkillMetadata(skillFile, name)
	return model.ToolSkill{
		Tool:          tool,
		Name:          name,
		DisplayName:   skillMetadata.Name,
		Description:   skillMetadata.Description,
		State:         model.SkillStateReadOnly,
		Source:        source,
		Group:         groupFromSource(source),
		EntryType:     entryType,
		ActivePath:    entryPath,
		SkillFilePath: skillFile,
		SymlinkTarget: symlinkTarget,
		ReadOnly:      true,
	}, true, nil
}

func disabledEntryToSkill(context *scanContext, entry state.DisabledEntry) (model.ToolSkill, error) {
	source := entry.Source
	if source == "" {
		source = model.SourceUnknown
	}
	group := entry.Group
	if group == "" {
		group = groupFromSource(source)
	}
	skillFile := ""
	skillMetadata := metadata.SkillMetadata{Name: entry.SkillName}
	if entry.DisabledPath != "" {
		candidateSkillFile := filepath.Join(entry.DisabledPath, "SKILL.md")
		if isRegularSkillFile(candidateSkillFile) {
			skillFile = candidateSkillFile
			skillMetadata = metadata.ReadSkillMetadata(candidateSkillFile, entry.SkillName)
		}
	}

	skill := model.ToolSkill{
		Tool:          entry.Tool,
		Name:          entry.SkillName,
		DisplayName:   skillMetadata.Name,
		Description:   skillMetadata.Description,
		State:         model.SkillStateOff,
		Source:        source,
		Group:         group,
		EntryType:     entry.EntryType,
		ActivePath:    entry.OriginalPath,
		DisabledPath:  entry.DisabledPath,
		SkillFilePath: skillFile,
		SymlinkTarget: entry.SymlinkTarget,
		ReadOnly:      false,
	}
	if entry.EntryType == model.EntryTypeSymlink {
		if skill.SymlinkTarget == "" {
			target, err := os.Readlink(entry.DisabledPath)
			if err == nil {
				skill.SymlinkTarget = target
			}
		}
		detectedSource, detectedGroup, repoOrigin, repoCommit := context.classifySymlinkSource(entry.DisabledPath)
		if source == model.SourceUnknown && detectedSource != model.SourceUnknown {
			skill.Source = detectedSource
		}
		if (skill.Group == "" || skill.Group == model.GroupUnknown) && detectedGroup != "" && detectedGroup != model.GroupUnknown {
			skill.Group = detectedGroup
		}
		skill.RepoOrigin = repoOrigin
		skill.RepoCommit = repoCommit
	}

	if entry.OriginalPath != "" {
		if _, err := os.Lstat(entry.OriginalPath); err == nil {
			skill.State = model.SkillStateConflict
			skill.Conflict = &model.Conflict{
				OriginalPath: entry.OriginalPath,
				DisabledPath: entry.DisabledPath,
				BlockerPath:  entry.OriginalPath,
				Message:      "Restore blocked because the original path already exists; move or remove the blocker manually, then rescan.",
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return model.ToolSkill{}, fmt.Errorf("inspect restore path %s: %w", entry.OriginalPath, err)
		}
	}

	return skill, nil
}

func isRegularSkillFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func isKnownTool(tool model.Tool) bool {
	_, ok := model.ParseTool(tool.String())
	return ok
}

func classifyManagedSource(context *scanContext, tool model.Tool, name, entryPath string, entryType model.EntryType, skillsCLINames map[string]struct{}) (model.SourceLabel, model.GroupLabel, string, string) {
	source := model.SourceUnknown
	group := model.GroupUnknown
	repoOrigin := ""
	repoCommit := ""

	switch entryType {
	case model.EntryTypeDir:
		source = model.SourceLocal
		group = model.GroupLocal
	case model.EntryTypeSymlink:
		if local, ok := context.localCells[tool.String()+"\x00"+name]; ok && !local.ambiguous && symlinkPointsTo(entryPath, local.target) {
			source = model.SourceLocalPath
			group = local.group
		} else {
			source, group, repoOrigin, repoCommit = context.classifySymlinkSource(entryPath)
		}
	}

	if tool == model.ToolCodex && source != model.SourceLocalPath {
		if _, ok := skillsCLINames[name]; ok {
			return model.SourceSkillsCLI, model.GroupSkillsCLI, repoOrigin, repoCommit
		}
	}

	if group == "" {
		group = groupFromSource(source)
	}
	return source, group, repoOrigin, repoCommit
}

func classifySymlinkSource(entryPath string) (model.SourceLabel, model.GroupLabel, string, string) {
	return newScanContext().classifySymlinkSource(entryPath)
}

func (context *scanContext) classifySymlinkSource(entryPath string) (model.SourceLabel, model.GroupLabel, string, string) {
	realPath, err := filepath.EvalSymlinks(entryPath)
	if err != nil {
		return model.SourceUnknown, model.GroupUnknown, "", ""
	}

	repoRoot := findLocalGitRoot(realPath)
	if repoRoot == "" {
		repoRoot = runGit(realPath, "rev-parse", "--show-toplevel")
	}
	if repoRoot == "" {
		return model.SourceUnknown, model.GroupUnknown, "", ""
	}
	repoRoot = filepath.Clean(repoRoot)

	if metadata, ok := context.reposByRoot[repoRoot]; ok {
		return model.SourceSymlinkRepo, metadata.group, metadata.origin, metadata.commit
	}

	origin := runGit(repoRoot, "config", "--get", "remote.origin.url")
	commit := runGit(repoRoot, "rev-parse", "--short", "HEAD")
	group := groupFromGit(origin, repoRoot)
	context.reposByRoot[repoRoot] = gitRepoMetadata{
		origin: origin,
		commit: commit,
		group:  group,
	}
	return model.SourceSymlinkRepo, group, origin, commit
}

func findLocalGitRoot(path string) string {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	}

	for {
		if hasGitMetadata(path) {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}

func hasGitMetadata(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode().IsRegular()
}

func groupFromSource(source model.SourceLabel) model.GroupLabel {
	switch source {
	case model.SourceSkillsCLI:
		return model.GroupSkillsCLI
	case model.SourceLocal:
		return model.GroupLocal
	case model.SourceLocalPath:
		return model.GroupLocal
	case model.SourceCodexSystem:
		return model.GroupCodexSystem
	case model.SourceClaudePlugin:
		return model.GroupClaudePlugin
	default:
		return model.GroupUnknown
	}
}

func symlinkPointsTo(linkPath, expectedTarget string) bool {
	target, err := os.Readlink(linkPath)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}
	return sameFilesystemPath(target, expectedTarget)
}

func sameFilesystemPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(filepath.Clean(left))
	rightAbs, rightErr := filepath.Abs(filepath.Clean(right))
	return leftErr == nil && rightErr == nil && leftAbs == rightAbs
}

func groupFromGit(origin, repoRoot string) model.GroupLabel {
	if origin != "" {
		if group, ok := githubGroupFromRemote(origin); ok {
			return group
		}
		if group, ok := fallbackRemoteGroup(origin); ok {
			return group
		}
	}

	if repoRoot != "" {
		if name := filepath.Base(repoRoot); name != "" && name != "." && name != string(filepath.Separator) {
			return model.GroupLabel(name)
		}
	}
	return model.GroupUnknown
}

func githubGroupFromRemote(origin string) (model.GroupLabel, bool) {
	host, repoPath, ok := remoteHostPath(origin)
	if !ok || !strings.EqualFold(host, "github.com") {
		return model.GroupUnknown, false
	}

	parts := strings.Split(cleanRemoteRepoPath(repoPath), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return model.GroupUnknown, false
	}
	return model.GroupLabel(parts[0] + "/" + parts[1]), true
}

func fallbackRemoteGroup(origin string) (model.GroupLabel, bool) {
	host, repoPath, ok := remoteHostPath(origin)
	if !ok {
		return model.GroupUnknown, false
	}

	repoPath = cleanRemoteRepoPath(repoPath)
	if host != "" && repoPath != "" {
		return model.GroupLabel(host + "/" + repoPath), true
	}
	if repoPath != "" {
		return model.GroupLabel(repoPath), true
	}
	return model.GroupUnknown, false
}

func remoteHostPath(origin string) (string, string, bool) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", "", false
	}

	if strings.Contains(origin, "://") {
		parsed, err := url.Parse(origin)
		if err != nil {
			return "", "", false
		}
		return strings.ToLower(parsed.Hostname()), parsed.Path, parsed.Path != ""
	}

	beforeColon, afterColon, foundColon := strings.Cut(origin, ":")
	if foundColon && strings.Contains(beforeColon, "@") {
		host := beforeColon[strings.LastIndex(beforeColon, "@")+1:]
		return strings.ToLower(host), afterColon, afterColon != ""
	}

	return "", origin, true
}

func cleanRemoteRepoPath(repoPath string) string {
	repoPath = strings.TrimSpace(repoPath)
	repoPath = strings.Trim(repoPath, "/")
	if strings.HasSuffix(repoPath, ".git") {
		repoPath = strings.TrimSuffix(repoPath, ".git")
	}
	return strings.Trim(repoPath, "/")
}

func runGit(dir string, args ...string) string {
	commandArgs := append([]string{"-C", dir}, args...)
	output, err := exec.Command("git", commandArgs...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// RowOptions controls how scanned skills are grouped for display.
type RowOptions struct {
	IncludeReadOnly bool
}

// RowsFromSkills groups tool-specific skill entries by skill name.
func RowsFromSkills(skills []model.ToolSkill) []model.SkillRow {
	return RowsFromSkillsWithOptions(skills, RowOptions{})
}

// RowsFromSkillsWithOptions groups tool-specific skill entries by skill name.
func RowsFromSkillsWithOptions(skills []model.ToolSkill, options RowOptions) []model.SkillRow {
	byName := make(map[string]*model.SkillRow)
	for i := range skills {
		skill := skills[i]
		if !options.IncludeReadOnly && !skill.Toggleable() {
			continue
		}

		row := byName[skill.Name]
		if row == nil {
			row = &model.SkillRow{
				Name:        skill.Name,
				Description: skill.Description,
				Source:      model.SourceUnknown,
				Group:       model.GroupUnknown,
			}
			byName[skill.Name] = row
		}

		if row.Description == "" {
			row.Description = skill.Description
		}

		switch skill.Tool {
		case model.ToolClaude:
			row.Claude = preferredCell(row.Claude, &skills[i])
		case model.ToolCodex:
			row.Codex = preferredCell(row.Codex, &skills[i])
		case model.ToolMuse:
			row.Muse = preferredCell(row.Muse, &skills[i])
		case model.ToolGrok:
			row.Grok = preferredCell(row.Grok, &skills[i])
		}
	}

	rows := make([]model.SkillRow, 0, len(byName))
	for _, row := range byName {
		row.Source = rowSource(*row)
		row.Group = rowGroup(*row)
		if !options.IncludeReadOnly && !rowHasToggleableCell(*row) {
			continue
		}
		rows = append(rows, *row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})

	return rows
}

func preferredCell(current, next *model.ToolSkill) *model.ToolSkill {
	if current == nil {
		return next
	}
	if cellPriority(next) > cellPriority(current) {
		return next
	}
	return current
}

func cellPriority(skill *model.ToolSkill) int {
	if skill == nil {
		return 0
	}
	if skill.State == model.SkillStateConflict {
		return 4
	}
	if skill.Toggleable() {
		return 3
	}
	if skill.ReadOnly || skill.State == model.SkillStateReadOnly {
		return 1
	}
	return 0
}

func rowHasToggleableCell(row model.SkillRow) bool {
	return (row.Claude != nil && row.Claude.Toggleable()) || (row.Codex != nil && row.Codex.Toggleable()) || (row.Muse != nil && row.Muse.Toggleable()) || (row.Grok != nil && row.Grok.Toggleable())
}

func rowSource(row model.SkillRow) model.SourceLabel {
	source := model.SourceUnknown
	if row.Claude != nil {
		source = mergeSource(source, row.Claude.Source)
	}
	if row.Codex != nil {
		source = mergeSource(source, row.Codex.Source)
	}
	if row.Muse != nil {
		source = mergeSource(source, row.Muse.Source)
	}
	if row.Grok != nil {
		source = mergeSource(source, row.Grok.Source)
	}
	return source
}

func rowGroup(row model.SkillRow) model.GroupLabel {
	group := model.GroupUnknown
	if row.Claude != nil {
		group = mergeGroup(group, row.Claude.Group)
	}
	if row.Codex != nil {
		group = mergeGroup(group, row.Codex.Group)
	}
	if row.Muse != nil {
		group = mergeGroup(group, row.Muse.Group)
	}
	if row.Grok != nil {
		group = mergeGroup(group, row.Grok.Group)
	}
	return group
}

func mergeSource(current, next model.SourceLabel) model.SourceLabel {
	if current == "" || current == model.SourceUnknown {
		return next
	}
	if next == "" || next == model.SourceUnknown || current == next {
		return current
	}
	return model.SourceUnknown
}

func mergeGroup(current, next model.GroupLabel) model.GroupLabel {
	if current == "" || current == model.GroupUnknown {
		if next == "" {
			return model.GroupUnknown
		}
		return next
	}
	if next == "" || next == model.GroupUnknown || current == next {
		return current
	}
	return model.GroupUnknown
}
