// Package gui provides the stateful, Wails-independent application service
// used by the macOS desktop interface.
package gui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dees91/agent-skill-manager/internal/contextbudget"
	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/ops"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/scan"
	"github.com/dees91/agent-skill-manager/internal/skillssh"
	"github.com/dees91/agent-skill-manager/internal/staging"
	"github.com/dees91/agent-skill-manager/internal/state"
)

type catalogClient interface {
	GetPage(context.Context, skillssh.View, int, bool) (skillssh.Page, error)
	Search(context.Context, string) (skillssh.Page, error)
	GetDetail(context.Context, skillssh.Skill, bool) (skillssh.Detail, error)
}

// Service owns one desktop session, including its in-memory pending changes.
type Service struct {
	mu              sync.Mutex
	paths           paths.Paths
	scanner         scan.Scanner
	operations      *ops.Service
	contextAnalyzer *contextbudget.Analyzer
	contextResult   contextbudget.Result
	pending         staging.Memory
	rows            []model.SkillRow
	managedSources  []ManagedSource
	includeReadOnly bool
	scannedAt       time.Time
	now             func() time.Time
	store           state.Store
	gitRunner       install.GitRunner
	catalog         catalogClient
	catalogSkills   map[string]skillssh.Skill
	drafts          map[string]installDraftState
	reviews         map[string]installReviewState
	sourceOperation sync.Mutex
	sourceBusy      atomic.Bool
	progressMu      sync.Mutex
	progress        func(SourceProgress)
	privacyReady    bool
}

// New creates a desktop session for the provided filesystem paths.
func New(p paths.Paths) *Service {
	return &Service{
		paths:           p,
		scanner:         scan.New(p),
		operations:      ops.New(p),
		contextAnalyzer: contextbudget.New(p),
		pending:         staging.Memory{},
		now:             time.Now,
		store:           state.New(p),
		catalog:         skillssh.New(p.SkillsSHCacheFile),
		catalogSkills:   map[string]skillssh.Skill{},
		drafts:          map[string]installDraftState{},
		reviews:         map[string]installReviewState{},
	}
}

// GetSnapshot rescans the filesystem and returns the complete GUI projection.
func (s *Service) GetSnapshot(includeReadOnly bool) (Snapshot, error) {
	if s.SourceBusy() {
		return Snapshot{}, fmt.Errorf("wait for the source operation to finish before refreshing")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return Snapshot{}, fmt.Errorf("wait for the source operation to finish before refreshing")
	}
	if !s.privacyReady {
		if err := s.store.Secure(); err != nil {
			return Snapshot{}, err
		}
		if err := skillssh.SanitizeCache(s.paths.SkillsSHCacheFile); err != nil {
			return Snapshot{}, err
		}
		s.privacyReady = true
	}
	if err := s.reloadLocked(includeReadOnly); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(), nil
}

// ToggleCell adds or removes the natural pending operation for one cell.
func (s *Service) ToggleCell(skillName, toolName string) (ActionResult, error) {
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	row, err := s.rowLocked(skillName)
	if err != nil {
		return ActionResult{}, err
	}
	tool, ok := model.ParseTool(strings.ToLower(strings.TrimSpace(toolName)))
	if !ok {
		return ActionResult{}, fmt.Errorf("unsupported tool %q", toolName)
	}

	result := staging.ToggleCell(s.pending, row, tool)
	message := "Cell cannot be toggled."
	if result.Removed {
		message = "Pending change removed."
	} else if result.Changed {
		message = "Pending change added."
	}
	return s.actionResultLocked(message, ActionCounts{
		Changed: boolCount(result.Changed && !result.Removed),
		Removed: boolCount(result.Removed),
	}), nil
}

// ToggleBoth toggles each existing tool cell in one row independently.
func (s *Service) ToggleBoth(skillName string) (ActionResult, error) {
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	row, err := s.rowLocked(skillName)
	if err != nil {
		return ActionResult{}, err
	}

	counts := ActionCounts{}
	for _, tool := range model.Tools() {
		key := staging.Key{Tool: tool, SkillName: row.Name}
		if _, exists := s.pending[key]; exists {
			result := staging.ToggleCell(s.pending, row, tool)
			if result.Removed {
				counts.Removed++
			}
			continue
		}
		cell := staging.SkillForTool(row, tool)
		switch {
		case cell == nil:
			counts.SkippedMissing++
			continue
		case cell.State == model.SkillStateConflict || cell.Conflict != nil:
			counts.SkippedConflict++
			continue
		case cell.ReadOnly || cell.State == model.SkillStateReadOnly:
			counts.SkippedReadOnly++
			continue
		}
		result := staging.ToggleCell(s.pending, row, tool)
		if result.Removed {
			counts.Removed++
		} else if result.Changed {
			counts.Changed++
		}
	}
	message := fmt.Sprintf("Row %s: %d pending change(s) updated.", row.Name, counts.Changed+counts.Removed)
	if counts.Changed+counts.Removed == 0 {
		message = "No toggleable cells in this row."
	}
	return s.actionResultLocked(message, counts), nil
}

// ToggleGroup smart-toggles every loaded row in a group across both tools.
func (s *Service) ToggleGroup(groupName string) (ActionResult, error) {
	return s.ToggleGroupScope(groupName, []string{model.ToolClaude.String(), model.ToolCodex.String()})
}

// ToggleGroupScope smart-toggles every loaded row in a group for the selected tools.
func (s *Service) ToggleGroupScope(groupName string, toolNames []string) (ActionResult, error) {
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return ActionResult{}, fmt.Errorf("group is required")
	}
	tools, err := parseToolScope(toolNames)
	if err != nil {
		return ActionResult{}, err
	}

	rows := make([]model.SkillRow, 0)
	for _, row := range s.rows {
		if normalizedGroup(row.Group).String() == groupName {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return ActionResult{}, fmt.Errorf("group %q not found", groupName)
	}
	batch := staging.ToggleBatch(s.pending, rows, tools)
	counts := countsFromBatch(batch)
	return s.actionResultLocked(formatBatchMessage("Group "+groupName, counts), counts), nil
}

// ToggleVisible smart-toggles the exact visible row names supplied by the UI.
func (s *Service) ToggleVisible(skillNames []string) (ActionResult, error) {
	return s.ToggleSkillScope(skillNames, []string{model.ToolClaude.String(), model.ToolCodex.String()})
}

// ToggleSkillScope smart-toggles exact skill names for the selected tools.
func (s *Service) ToggleSkillScope(skillNames, toolNames []string) (ActionResult, error) {
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	if len(skillNames) == 0 {
		return s.actionResultLocked("No matching skills.", ActionCounts{}), nil
	}
	tools, err := parseToolScope(toolNames)
	if err != nil {
		return ActionResult{}, err
	}

	byName := make(map[string]model.SkillRow, len(s.rows))
	for _, row := range s.rows {
		byName[row.Name] = row
	}
	seen := make(map[string]struct{}, len(skillNames))
	rows := make([]model.SkillRow, 0, len(skillNames))
	for _, name := range skillNames {
		name = strings.TrimSpace(name)
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		row, ok := byName[name]
		if !ok {
			return ActionResult{}, fmt.Errorf("visible skill %q is not in the current snapshot", name)
		}
		seen[name] = struct{}{}
		rows = append(rows, row)
	}

	batch := staging.ToggleBatch(s.pending, rows, tools)
	counts := countsFromBatch(batch)
	return s.actionResultLocked(formatBatchMessage("Filtered results", counts), counts), nil
}

// UndoCell removes one pending operation if present.
func (s *Service) UndoCell(skillName, toolName string) (ActionResult, error) {
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	tool, ok := model.ParseTool(strings.ToLower(strings.TrimSpace(toolName)))
	if !ok {
		return ActionResult{}, fmt.Errorf("unsupported tool %q", toolName)
	}
	key := staging.Key{Tool: tool, SkillName: strings.TrimSpace(skillName)}
	if _, ok := s.pending[key]; !ok {
		return s.actionResultLocked("No pending change for this cell.", ActionCounts{}), nil
	}
	delete(s.pending, key)
	return s.actionResultLocked("Pending change removed.", ActionCounts{Removed: 1}), nil
}

// ClearPending removes every pending operation in the desktop session.
func (s *Service) ClearPending() ActionResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return s.actionResultLocked("A source operation is in progress.", ActionCounts{})
	}
	removed := len(s.pending)
	s.pending = staging.Memory{}
	message := "No pending changes to clear."
	if removed > 0 {
		message = "All pending changes cleared."
	}
	return s.actionResultLocked(message, ActionCounts{Removed: removed})
}

// ApplyPending preflights and applies the current pending set, then rescans.
func (s *Service) ApplyPending(includeReadOnly bool) ApplyResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return ApplyResult{Message: "A source operation is in progress.", Failure: &ApplyFailure{Stage: "busy", Message: "wait for the source operation to finish"}, Snapshot: s.snapshotLocked()}
	}

	changes := s.pendingChangesLocked()
	if len(changes) == 0 {
		return ApplyResult{Message: "No pending changes to apply.", Snapshot: s.snapshotLocked()}
	}

	requests := make([]ops.PlanRequest, 0, len(changes))
	for _, change := range changes {
		requests = append(requests, ops.PlanRequest{
			Kind:      model.OperationKind(change.Operation),
			Tool:      model.Tool(change.Tool),
			SkillName: change.SkillName,
		})
	}
	planned, err := s.operations.PlanBatch(requests)
	if err != nil {
		_ = s.reloadLocked(includeReadOnly)
		return ApplyResult{
			Message:  "Cannot apply pending changes.",
			Failure:  &ApplyFailure{Stage: "preflight", Message: err.Error()},
			Snapshot: s.snapshotLocked(),
		}
	}

	applied := s.operations.Apply(planned)
	completed := make([]AppliedChange, 0, len(applied.Completed))
	for _, operation := range applied.Completed {
		delete(s.pending, staging.Key{Tool: operation.Tool, SkillName: operation.SkillName})
		completed = append(completed, AppliedChange{
			Tool:      operation.Tool.String(),
			SkillName: operation.SkillName,
			Operation: operation.Kind.String(),
		})
	}

	reloadErr := s.reloadLocked(includeReadOnly)
	result := ApplyResult{
		Completed: completed,
		Message:   fmt.Sprintf("Applied %d change(s).", len(completed)),
		Snapshot:  s.snapshotLocked(),
	}
	if applied.Failed != nil {
		failure := applied.Failed.Operation
		result.Message = "Apply stopped after the first failure."
		result.Failure = &ApplyFailure{
			Stage:     "apply",
			Tool:      failure.Tool.String(),
			SkillName: failure.SkillName,
			Operation: failure.Kind.String(),
			Message:   applied.Failed.Err.Error(),
		}
	}
	if reloadErr != nil {
		result.Message = "Changes applied, but the follow-up scan failed."
		result.Failure = &ApplyFailure{Stage: "rescan", Message: reloadErr.Error()}
	}
	return result
}

// PendingCount returns the number of unsaved session changes.
func (s *Service) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// SourceBusy reports whether a source lifecycle operation owns the mutation lane.
func (s *Service) SourceBusy() bool { return s.sourceBusy.Load() }

// SetSourceProgressHandler installs the Wails-independent progress callback.
func (s *Service) SetSourceProgressHandler(handler func(SourceProgress)) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.progress = handler
}

func (s *Service) reloadLocked(includeReadOnly bool) error {
	skills, err := s.scanner.Managed()
	if err == nil {
		var disabled []model.ToolSkill
		disabled, err = s.scanner.Disabled()
		skills = append(skills, disabled...)
	}
	if err == nil && includeReadOnly {
		var readOnly []model.ToolSkill
		readOnly, err = s.scanner.ReadOnly()
		skills = append(skills, readOnly...)
	}
	if err != nil {
		return err
	}
	s.rows = scan.RowsFromSkillsWithOptions(skills, scan.RowOptions{IncludeReadOnly: includeReadOnly})
	manifest, manifestErr := s.store.Load()
	if manifestErr != nil {
		return manifestErr
	}
	s.managedSources = projectManagedSources(manifest)
	s.contextResult = s.contextAnalyzer.Estimate(s.rows)
	s.includeReadOnly = includeReadOnly
	s.scannedAt = s.now().UTC()
	return nil
}

// MeasureContextBudgets explicitly runs the supported provider diagnostics and
// returns a fresh projection while preserving process-local pending changes.
func (s *Service) MeasureContextBudgets() (Snapshot, error) {
	if s.SourceBusy() {
		return Snapshot{}, fmt.Errorf("wait for the source operation to finish before running diagnostics")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scannedAt.IsZero() {
		if err := s.reloadLocked(false); err != nil {
			return Snapshot{}, err
		}
	}
	s.contextResult = s.contextAnalyzer.Measure(s.rows)
	return s.snapshotLocked(), nil
}

func (s *Service) rowLocked(name string) (model.SkillRow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.SkillRow{}, fmt.Errorf("skill name is required")
	}
	if s.scannedAt.IsZero() {
		if err := s.reloadLocked(false); err != nil {
			return model.SkillRow{}, err
		}
	}
	for _, row := range s.rows {
		if row.Name == name {
			return row, nil
		}
	}
	return model.SkillRow{}, fmt.Errorf("skill %q not found in the current snapshot", name)
}

func (s *Service) snapshotLocked() Snapshot {
	rows := make([]SkillRow, 0, len(s.rows))
	for _, row := range s.rows {
		rows = append(rows, projectRow(row, s.pending))
	}
	groups := projectGroups(scan.GroupSummaries(s.rows))
	stats, conflicts := summarize(s.rows)
	return Snapshot{
		Rows:            rows,
		Groups:          groups,
		Sources:         collectSources(s.rows),
		ManagedSources:  append([]ManagedSource{}, s.managedSources...),
		Stats:           stats,
		Conflicts:       conflicts,
		ContextBudgets:  s.contextResult.Project(s.contextPendingLocked()),
		Pending:         s.pendingChangesLocked(),
		IncludeReadOnly: s.includeReadOnly,
		ScannedAt:       formatTime(s.scannedAt),
	}
}

func (s *Service) actionResultLocked(message string, counts ActionCounts) ActionResult {
	return ActionResult{
		Message:        message,
		Counts:         counts,
		Pending:        s.pendingChangesLocked(),
		ContextBudgets: s.contextResult.Project(s.contextPendingLocked()),
	}
}

func (s *Service) contextPendingLocked() map[contextbudget.CellKey]model.OperationKind {
	projected := make(map[contextbudget.CellKey]model.OperationKind, len(s.pending))
	for key, operation := range s.pending {
		projected[contextbudget.CellKey{Tool: key.Tool, SkillName: key.SkillName}] = operation
	}
	return projected
}

func (s *Service) pendingChangesLocked() []PendingChange {
	changes := make([]PendingChange, 0, len(s.pending))
	for key, kind := range s.pending {
		changes = append(changes, PendingChange{
			Tool:      key.Tool.String(),
			SkillName: key.SkillName,
			Operation: kind.String(),
		})
	}
	sort.SliceStable(changes, func(i, j int) bool {
		leftRank := operationRank(changes[i].Operation)
		rightRank := operationRank(changes[j].Operation)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if changes[i].Tool != changes[j].Tool {
			return changes[i].Tool < changes[j].Tool
		}
		return changes[i].SkillName < changes[j].SkillName
	})
	return changes
}

func operationRank(kind string) int {
	if kind == model.OperationDisable.String() {
		return 0
	}
	if kind == model.OperationEnable.String() {
		return 1
	}
	return 2
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func parseToolScope(names []string) ([]model.Tool, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("at least one tool is required")
	}
	requested := make(map[model.Tool]struct{}, len(names))
	for _, name := range names {
		tool, ok := model.ParseTool(strings.ToLower(strings.TrimSpace(name)))
		if !ok {
			return nil, fmt.Errorf("unsupported tool %q", name)
		}
		requested[tool] = struct{}{}
	}
	tools := make([]model.Tool, 0, len(requested))
	for _, tool := range model.Tools() {
		if _, ok := requested[tool]; ok {
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

func countsFromBatch(result staging.BatchResult) ActionCounts {
	return ActionCounts{
		Changed:         result.Changed,
		Removed:         result.Removed,
		SkippedReadOnly: result.SkippedReadOnly,
		SkippedMissing:  result.SkippedMissing,
		SkippedConflict: result.SkippedConflict,
	}
}

func formatBatchMessage(scope string, counts ActionCounts) string {
	updated := counts.Changed + counts.Removed
	if updated == 0 {
		return scope + ": no applicable cells."
	}
	return fmt.Sprintf("%s: %d pending change(s) updated.", scope, updated)
}

func normalizedGroup(group model.GroupLabel) model.GroupLabel {
	if group == "" {
		return model.GroupUnknown
	}
	return group
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
