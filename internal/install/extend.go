package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/ops"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// ExtendSourceKind identifies the managed source kind in extend plans and
// results. The tool being extended to is always a parameter, never a kind.
type ExtendSourceKind string

const (
	ExtendSourceGit   ExtendSourceKind = "git"
	ExtendSourceLocal ExtendSourceKind = "local"
)

// ExtendSourceStatus describes one source inside an extend plan or result.
type ExtendSourceStatus string

const (
	ExtendStatusReady     ExtendSourceStatus = "ready"
	ExtendStatusUnchanged ExtendSourceStatus = "unchanged"
	ExtendStatusSkipped   ExtendSourceStatus = "skipped"
	ExtendStatusBlocked   ExtendSourceStatus = "blocked"
	ExtendStatusExtended  ExtendSourceStatus = "extended"
)

// ExtendSkip explains why one recorded skill was not planned for the target.
type ExtendSkip struct {
	SkillName string
	Reason    string
}

// ExtendSourcePlan is the per-source portion of an extend plan. Links and
// AlreadyInstalled come from the shared install planner, so extend stays
// idempotent and repairs missing recorded links exactly like install.
type ExtendSourcePlan struct {
	Kind             ExtendSourceKind
	Group            model.GroupLabel
	Location         string
	Status           ExtendSourceStatus
	Reason           string
	Err              error
	Links            []LinkPlan
	AlreadyInstalled []AlreadyInstalled
	Skipped          []ExtendSkip
	DisableAfter     []string

	gitPlan        InstallPlan
	localPlan      LocalInstallPlan
	lastSeenCommit string
}

// ExtendPlan links every recorded skill of every managed source to one tool.
type ExtendPlan struct {
	Tool    model.Tool
	Sources []ExtendSourcePlan
}

// ExtendTotals aggregates an extend plan for dry-run and preview output.
type ExtendTotals struct {
	Sources          int
	Ready            int
	Unchanged        int
	SkippedSources   int
	Blocked          int
	Links            int
	AlreadyInstalled int
	AlreadyOn        int
	AlreadyOff       int
	DisableAfter     int
	SkippedSkills    int
}

// Totals aggregates plan counts across all sources.
func (p ExtendPlan) Totals() ExtendTotals {
	totals := ExtendTotals{Sources: len(p.Sources)}
	for _, source := range p.Sources {
		switch source.Status {
		case ExtendStatusReady:
			totals.Ready++
		case ExtendStatusUnchanged:
			totals.Unchanged++
		case ExtendStatusSkipped:
			totals.SkippedSources++
		case ExtendStatusBlocked:
			totals.Blocked++
		}
		totals.Links += len(source.Links)
		totals.AlreadyInstalled += len(source.AlreadyInstalled)
		for _, already := range source.AlreadyInstalled {
			if already.State == model.SkillStateOff {
				totals.AlreadyOff++
			} else {
				totals.AlreadyOn++
			}
		}
		totals.DisableAfter += len(source.DisableAfter)
		totals.SkippedSkills += len(source.Skipped)
	}
	return totals
}

// PlanExtend plans links for one target tool across every recorded managed
// source in manifest order. It never clones, fetches, or mutates state.
func PlanExtend(p paths.Paths, manifest state.Manifest, tool model.Tool) (ExtendPlan, error) {
	if _, ok := p.UserSkillsDirFor(tool); !ok {
		return ExtendPlan{}, fmt.Errorf("unsupported tool %q", tool)
	}
	plan := ExtendPlan{Tool: tool, Sources: []ExtendSourcePlan{}}
	planned := map[string]string{}
	for i := range manifest.Repositories {
		plan.Sources = append(plan.Sources, planExtendRepository(p, manifest, tool, manifest.Repositories[i], planned))
	}
	for i := range manifest.LocalSources {
		plan.Sources = append(plan.Sources, planExtendLocalSource(p, manifest, tool, manifest.LocalSources[i], planned))
	}
	return plan, nil
}

func planExtendRepository(p paths.Paths, manifest state.Manifest, tool model.Tool, repo state.RepositoryEntry, planned map[string]string) ExtendSourcePlan {
	source := ExtendSourcePlan{
		Kind:     ExtendSourceGit,
		Group:    repo.Group,
		Location: repo.OriginalURL,
		Skipped:  []ExtendSkip{},
	}
	if source.Location == "" {
		source.Location = repo.CanonicalURL
	}
	identity, checkoutPath, err := validateRecordedRepository(p, repo)
	if err != nil {
		source.Status = ExtendStatusBlocked
		source.Err = err
		return source
	}
	if _, err := os.Stat(checkoutPath); err != nil {
		if os.IsNotExist(err) {
			source.Status = ExtendStatusSkipped
			source.Reason = "checkout is missing"
			return source
		}
		source.Status = ExtendStatusBlocked
		source.Err = fmt.Errorf("inspect checkout %s: %w", checkoutPath, err)
		return source
	}
	discovered, err := DiscoverSkills(checkoutPath)
	if err != nil {
		source.Status = ExtendStatusBlocked
		source.Err = err
		return source
	}
	cells, disableAfter, skipped := extendCells(manifest, tool, repo.InstalledSkills, discovered)
	source.Skipped = skipped
	source.DisableAfter = disableAfter
	if len(cells) == 0 {
		source.Status = ExtendStatusUnchanged
		source.Reason = fmt.Sprintf("no links to create for %s", tool)
		return source
	}
	if err := claimExtendTargets(tool, cells, repo.Group.String(), planned); err != nil {
		source.Status = ExtendStatusBlocked
		source.Err = err
		return source
	}
	installPlan, err := PlanInstall(p, manifest, identity, checkoutPath, discovered, PlanOptions{Cells: cells})
	if err != nil {
		source.Status = ExtendStatusBlocked
		source.Err = err
		return source
	}
	source.Status = ExtendStatusReady
	source.Links = installPlan.Links
	source.AlreadyInstalled = installPlan.AlreadyInstalled
	source.gitPlan = installPlan
	source.lastSeenCommit = repo.LastSeenCommit
	return source
}

func planExtendLocalSource(p paths.Paths, manifest state.Manifest, tool model.Tool, entry state.LocalSourceEntry, planned map[string]string) ExtendSourcePlan {
	source := ExtendSourcePlan{
		Kind:     ExtendSourceLocal,
		Group:    entry.Group,
		Location: entry.CanonicalPath,
		Skipped:  []ExtendSkip{},
	}
	lookup, err := ResolveLocalSourceLookup(p, "/", entry.OriginalPath)
	if err != nil {
		source.Status = ExtendStatusBlocked
		source.Err = err
		return source
	}
	if !lookup.Exists {
		source.Status = ExtendStatusSkipped
		source.Reason = "local source directory is missing"
		return source
	}
	localSource, err := ResolveLocalSource(p, "/", entry.OriginalPath)
	if err != nil {
		source.Status = ExtendStatusBlocked
		source.Err = err
		return source
	}
	if localSource.CanonicalPath != entry.CanonicalPath {
		source.Status = ExtendStatusBlocked
		source.Err = fmt.Errorf("local source moved from %s", entry.CanonicalPath)
		return source
	}
	discovered, err := DiscoverLocalSkills(localSource)
	if err != nil {
		source.Status = ExtendStatusBlocked
		source.Err = err
		return source
	}
	cells, disableAfter, skipped := extendCells(manifest, tool, entry.InstalledSkills, discovered)
	source.Skipped = skipped
	source.DisableAfter = disableAfter
	if len(cells) == 0 {
		source.Status = ExtendStatusUnchanged
		source.Reason = fmt.Sprintf("no links to create for %s", tool)
		return source
	}
	if err := claimExtendTargets(tool, cells, entry.Group.String(), planned); err != nil {
		source.Status = ExtendStatusBlocked
		source.Err = err
		return source
	}
	localPlan, err := PlanLocalInstall(p, manifest, localSource, discovered, PlanOptions{Cells: cells})
	if err != nil {
		source.Status = ExtendStatusBlocked
		source.Err = err
		return source
	}
	source.Status = ExtendStatusReady
	source.Links = localPlan.Links
	source.AlreadyInstalled = localPlan.AlreadyInstalled
	source.localPlan = localPlan
	return source
}

// extendCells derives exact install cells for the target tool from recorded
// ownership. Skills already recorded for the target stay idempotent cells;
// skills ON for another tool end ON; skills OFF everywhere are linked first
// and disabled afterwards.
func extendCells(manifest state.Manifest, tool model.Tool, installed []state.InstalledSkillEntry, discovered []DiscoveredSkill) ([]InstallCell, []string, []ExtendSkip) {
	byRelative := make(map[string]DiscoveredSkill, len(discovered))
	for _, skill := range discovered {
		byRelative[normalizeRecordedRelativePath(skill.RelativePath)] = skill
	}
	cells := []InstallCell{}
	disableAfter := []string{}
	skipped := []ExtendSkip{}
	for _, entry := range installed {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		if hasRecordedTool(entry.Tools, tool) {
			cells = append(cells, InstallCell{SkillName: name, Tool: tool})
			continue
		}
		if len(recordedTools(entry.Tools)) == 0 {
			skipped = append(skipped, ExtendSkip{SkillName: name, Reason: "no recorded tools"})
			continue
		}
		if _, ok := byRelative[normalizeRecordedRelativePath(entry.RelativePath)]; !ok {
			skipped = append(skipped, ExtendSkip{SkillName: name, Reason: fmt.Sprintf("not found in source at recorded path %s", entry.RelativePath)})
			continue
		}
		cells = append(cells, InstallCell{SkillName: name, Tool: tool})
		if extendEndsOn(manifest, entry, tool) {
			continue
		}
		disableAfter = append(disableAfter, name)
	}
	return cells, disableAfter, skipped
}

func normalizeRecordedRelativePath(relative string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(relative)))
}

func hasRecordedTool(tools []model.Tool, tool model.Tool) bool {
	for _, recorded := range tools {
		if recorded == tool {
			return true
		}
	}
	return false
}

func recordedTools(tools []model.Tool) []model.Tool {
	kept := make([]model.Tool, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.String()) != "" {
			kept = append(kept, tool)
		}
	}
	return kept
}

// extendEndsOn reports whether the extended cell should stay ON. A recorded
// skill ends ON when at least one other recorded tool has no disabled entry.
func extendEndsOn(manifest state.Manifest, entry state.InstalledSkillEntry, tool model.Tool) bool {
	for _, other := range entry.Tools {
		if other == tool {
			continue
		}
		if _, ok := manifest.Get(other, entry.Name); !ok {
			return true
		}
	}
	return false
}

// claimExtendTargets reserves (tool, skill) targets across sources in manifest
// order so a later source planning the same cell is blocked instead of
// racing the first one.
func claimExtendTargets(tool model.Tool, cells []InstallCell, group string, planned map[string]string) error {
	for _, cell := range cells {
		key := repositoryCellKey(tool, cell.SkillName)
		if owner, ok := planned[key]; ok {
			return fmt.Errorf("target %s/%s is planned for %s", tool, cell.SkillName, owner)
		}
		planned[key] = group
	}
	return nil
}

// ExtendProgress reports extend apply progress per source.
type ExtendProgress struct {
	Kind    ExtendSourceKind
	Group   model.GroupLabel
	Current int
	Total   int
}

// ExtendSourceResult describes one source completed by extend apply.
type ExtendSourceResult struct {
	Kind             ExtendSourceKind
	Group            model.GroupLabel
	Location         string
	Status           ExtendSourceStatus
	Reason           string
	Created          int
	AlreadyInstalled int
	Disabled         int
	Skipped          []ExtendSkip
}

// ExtendResult aggregates extend apply across sources in manifest order.
type ExtendResult struct {
	Tool       model.Tool
	Completed  []ExtendSourceResult
	RolledBack int
}

// ExtendFailure stops extend apply at the first failed source while keeping
// the completed prefix reflected in state.
type ExtendFailure struct {
	Kind  ExtendSourceKind
	Group model.GroupLabel
	Tool  model.Tool
	Err   error
}

func (e *ExtendFailure) Error() string {
	return fmt.Sprintf("extend --tool %s failed for source %s: %v", e.Tool, e.Group, e.Err)
}

func (e *ExtendFailure) Unwrap() error {
	return e.Err
}

// ExtendService applies extend plans source by source with one state backup
// per apply service, mirroring the stop-on-first-failure update-all contract.
type ExtendService struct {
	paths paths.Paths
	store state.Store
	git   *ApplyService
	local *LocalApplyService
	ops   *ops.Service
}

// NewExtendService creates an extend service for the provided paths.
func NewExtendService(p paths.Paths) *ExtendService {
	return &ExtendService{
		paths: p,
		store: state.New(p),
		git:   NewApplyService(p),
		local: NewLocalApplyService(p),
		ops:   ops.New(p),
	}
}

// Apply links every recorded skill to the target tool in manifest order and
// stops at the first failed source, keeping the completed prefix.
func (s *ExtendService) Apply(tool model.Tool, progress func(ExtendProgress)) (ExtendResult, error) {
	result := ExtendResult{Tool: tool, Completed: []ExtendSourceResult{}}
	if _, ok := s.paths.UserSkillsDirFor(tool); !ok {
		return result, fmt.Errorf("unsupported tool %q", tool)
	}
	manifest, err := s.store.Load()
	if err != nil {
		return result, err
	}
	total := len(manifest.Repositories) + len(manifest.LocalSources)
	planned := map[string]string{}
	current := 0
	report := func(kind ExtendSourceKind, group model.GroupLabel) {
		current++
		if progress != nil {
			progress(ExtendProgress{Kind: kind, Group: group, Current: current, Total: total})
		}
	}
	for _, recorded := range manifest.Repositories {
		report(ExtendSourceGit, recorded.Group)
		manifest, err = s.store.Load()
		if err != nil {
			return result, err
		}
		repo, ok := manifest.GetRepository(recorded.Host, recorded.RepoPath)
		if !ok {
			result.Completed = append(result.Completed, ExtendSourceResult{Kind: ExtendSourceGit, Group: recorded.Group, Location: recorded.OriginalURL, Status: ExtendStatusSkipped, Reason: "source is no longer recorded"})
			continue
		}
		source := planExtendRepository(s.paths, manifest, tool, repo, planned)
		done, rolledBack, failure := s.applySource(tool, source, repo.LastSeenCommit)
		result.Completed = append(result.Completed, done...)
		result.RolledBack += rolledBack
		if failure != nil {
			return result, failure
		}
	}
	for _, recorded := range manifest.LocalSources {
		report(ExtendSourceLocal, recorded.Group)
		manifest, err = s.store.Load()
		if err != nil {
			return result, err
		}
		entry, ok := manifest.GetLocalSource(recorded.CanonicalPath)
		if !ok {
			result.Completed = append(result.Completed, ExtendSourceResult{Kind: ExtendSourceLocal, Group: recorded.Group, Location: recorded.CanonicalPath, Status: ExtendStatusSkipped, Reason: "source is no longer recorded"})
			continue
		}
		source := planExtendLocalSource(s.paths, manifest, tool, entry, planned)
		done, rolledBack, failure := s.applySource(tool, source, "")
		result.Completed = append(result.Completed, done...)
		result.RolledBack += rolledBack
		if failure != nil {
			return result, failure
		}
	}
	return result, nil
}

func (s *ExtendService) applySource(tool model.Tool, source ExtendSourcePlan, lastSeenCommit string) ([]ExtendSourceResult, int, error) {
	base := ExtendSourceResult{Kind: source.Kind, Group: source.Group, Location: source.Location, Skipped: source.Skipped}
	switch source.Status {
	case ExtendStatusSkipped, ExtendStatusUnchanged:
		base.Status = source.Status
		base.Reason = source.Reason
		return []ExtendSourceResult{base}, 0, nil
	case ExtendStatusBlocked:
		return nil, 0, &ExtendFailure{Kind: source.Kind, Group: source.Group, Tool: tool, Err: source.Err}
	}
	var created, already int
	var rolledBack int
	if source.Kind == ExtendSourceGit {
		commit := lastSeenCommit
		if commit == "" {
			commit = source.lastSeenCommit
		}
		applied, err := s.git.Apply(source.gitPlan, commit)
		rolledBack = len(applied.RolledBack)
		if err != nil {
			return nil, rolledBack, &ExtendFailure{Kind: source.Kind, Group: source.Group, Tool: tool, Err: err}
		}
		created = len(applied.Created)
		already = len(applied.AlreadyInstalled)
	} else {
		applied, err := s.local.Apply(source.localPlan)
		rolledBack = len(applied.RolledBack)
		if err != nil {
			return nil, rolledBack, &ExtendFailure{Kind: source.Kind, Group: source.Group, Tool: tool, Err: err}
		}
		created = len(applied.Created)
		already = len(applied.AlreadyInstalled)
	}
	disabled, err := s.disableAfter(tool, source.Kind, source.Group, source.DisableAfter)
	if err != nil {
		return nil, rolledBack, err
	}
	base.Status = ExtendStatusExtended
	base.Created = created
	base.AlreadyInstalled = already
	base.Disabled = disabled
	return []ExtendSourceResult{base}, rolledBack, nil
}

// disableAfter mirrors OFF state for skills that are disabled for every other
// recorded tool. Created links stay ON when a disable fails so the operator
// can toggle them manually.
func (s *ExtendService) disableAfter(tool model.Tool, kind ExtendSourceKind, group model.GroupLabel, names []string) (int, error) {
	if len(names) == 0 {
		return 0, nil
	}
	operations := make([]model.PlannedOperation, 0, len(names))
	for _, name := range names {
		operation, err := s.ops.PlanDisable(tool, name)
		if err != nil {
			return 0, &ExtendFailure{Kind: kind, Group: group, Tool: tool, Err: fmt.Errorf("disable %s: %w (created links stay ON and can be toggled)", name, err)}
		}
		operations = append(operations, operation)
	}
	applied := s.ops.Apply(operations)
	if applied.Failed != nil {
		return 0, &ExtendFailure{Kind: kind, Group: group, Tool: tool, Err: fmt.Errorf("disable %s: %v (created links stay ON and can be toggled)", applied.Failed.Operation.SkillName, applied.Failed.Err)}
	}
	return len(applied.Completed), nil
}
