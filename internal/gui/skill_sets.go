package gui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/skillsets"
	"github.com/dees91/agent-skill-manager/internal/staging"
	"github.com/dees91/agent-skill-manager/internal/state"
)

const (
	skillSetStatusEnabled        = "enabled"
	skillSetStatusDisabled       = "disabled"
	skillSetStatusMixed          = "mixed"
	skillSetStatusUnavailable    = "unavailable"
	skillSetStatusNeedsAttention = "needs-attention"
)

// CreateSkillSet saves one tool-agnostic recipe from currently known skills.
func (s *Service) CreateSkillSet(name, description string, skillNames []string) (SkillSetMutationResult, error) {
	if s.SourceBusy() {
		return SkillSetMutationResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return SkillSetMutationResult{}, fmt.Errorf("a source operation is in progress")
	}
	if err := s.ensureLoadedLocked(); err != nil {
		return SkillSetMutationResult{}, err
	}
	normalized, err := s.validateSkillSetMembersLocked(skillNames, nil)
	if err != nil {
		return SkillSetMutationResult{}, err
	}
	created, err := s.skillSetStore.Create(name, description, normalized)
	if err != nil {
		return SkillSetMutationResult{}, err
	}
	if err := s.refreshSkillSetsLocked(); err != nil {
		return SkillSetMutationResult{}, err
	}
	return s.skillSetMutationResultLocked(fmt.Sprintf("Created Skill Set %s.", created.Name)), nil
}

// UpdateSkillSet replaces one saved recipe while retaining already-recorded
// members that are currently unavailable.
func (s *Service) UpdateSkillSet(setID, name, description string, skillNames []string) (SkillSetMutationResult, error) {
	if s.SourceBusy() {
		return SkillSetMutationResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return SkillSetMutationResult{}, fmt.Errorf("a source operation is in progress")
	}
	if err := s.ensureLoadedLocked(); err != nil {
		return SkillSetMutationResult{}, err
	}
	existing, err := s.skillSetByIDLocked(setID)
	if err != nil {
		return SkillSetMutationResult{}, err
	}
	allowedMissing := make(map[string]bool, len(existing.Skills))
	for _, skillName := range existing.Skills {
		allowedMissing[skillName] = true
	}
	normalized, err := s.validateSkillSetMembersLocked(skillNames, allowedMissing)
	if err != nil {
		return SkillSetMutationResult{}, err
	}
	updated, err := s.skillSetStore.Update(existing.ID, name, description, normalized)
	if err != nil {
		return SkillSetMutationResult{}, err
	}
	if err := s.refreshSkillSetsLocked(); err != nil {
		return SkillSetMutationResult{}, err
	}
	return s.skillSetMutationResultLocked(fmt.Sprintf("Updated Skill Set %s.", updated.Name)), nil
}

// DeleteSkillSet removes recipe metadata only. Existing pending operations are
// intentionally independent and remain available for Apply or undo.
func (s *Service) DeleteSkillSet(setID string) (SkillSetMutationResult, error) {
	if s.SourceBusy() {
		return SkillSetMutationResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return SkillSetMutationResult{}, fmt.Errorf("a source operation is in progress")
	}
	if err := s.ensureLoadedLocked(); err != nil {
		return SkillSetMutationResult{}, err
	}
	existing, err := s.skillSetByIDLocked(setID)
	if err != nil {
		return SkillSetMutationResult{}, err
	}
	if _, err := s.skillSetStore.Delete(existing.ID); err != nil {
		return SkillSetMutationResult{}, err
	}
	if err := s.refreshSkillSetsLocked(); err != nil {
		return SkillSetMutationResult{}, err
	}
	return s.skillSetMutationResultLocked(fmt.Sprintf("Deleted Skill Set %s. Pending skill changes were not modified.", existing.Name)), nil
}

// PreviewSkillSetToggle calculates the existing smart-toggle effect using a
// copy of Pending. It does not mutate session or filesystem state.
func (s *Service) PreviewSkillSetToggle(setID string, toolNames []string) (SkillSetTogglePreview, error) {
	if s.SourceBusy() {
		return SkillSetTogglePreview{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return SkillSetTogglePreview{}, err
	}
	set, err := s.skillSetByIDLocked(setID)
	if err != nil {
		return SkillSetTogglePreview{}, err
	}
	tools, err := parseToolScope(toolNames)
	if err != nil {
		return SkillSetTogglePreview{}, err
	}
	rows := s.skillSetRowsLocked(set)
	direction, eligible := skillSetDirection(rows, tools, s.pending)
	copyPending := make(staging.Memory, len(s.pending))
	for key, operation := range s.pending {
		copyPending[key] = operation
	}
	batch := staging.ToggleBatch(copyPending, rows, tools)
	return SkillSetTogglePreview{
		SetID: set.ID, Name: set.Name, Tools: toolStrings(tools), Direction: direction,
		Eligible: eligible, Counts: countsFromBatch(batch),
	}, nil
}

// ToggleSkillSet stages one scoped smart-toggle through the shared engine.
func (s *Service) ToggleSkillSet(setID string, toolNames []string) (ActionResult, error) {
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return ActionResult{}, fmt.Errorf("a source operation is in progress")
	}
	if err := s.ensureLoadedLocked(); err != nil {
		return ActionResult{}, err
	}
	set, err := s.skillSetByIDLocked(setID)
	if err != nil {
		return ActionResult{}, err
	}
	tools, err := parseToolScope(toolNames)
	if err != nil {
		return ActionResult{}, err
	}
	batch := staging.ToggleBatch(s.pending, s.skillSetRowsLocked(set), tools)
	counts := countsFromBatch(batch)
	return s.actionResultLocked(formatBatchMessage("Skill Set "+set.Name, counts), counts), nil
}

func (s *Service) ensureLoadedLocked() error {
	if s.scannedAt.IsZero() {
		if err := s.reloadLocked(false); err != nil {
			return err
		}
	}
	if s.skillSetsWarning != "" {
		return fmt.Errorf("Skill Sets are unavailable: %s", s.skillSetsWarning)
	}
	return nil
}

func (s *Service) refreshSkillSetsLocked() error {
	file, err := s.skillSetStore.Load()
	if err != nil {
		s.skillSetFile = skillsets.File{}
		s.skillSetsWarning = err.Error()
		return err
	}
	s.skillSetFile = file
	s.skillSetsWarning = ""
	return nil
}

func (s *Service) reloadSkillSetsLocked() {
	if err := s.refreshSkillSetsLocked(); err != nil {
		return
	}
}

func (s *Service) skillSetByIDLocked(id string) (skillsets.Set, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return skillsets.Set{}, fmt.Errorf("Skill Set id is required")
	}
	set, ok := s.skillSetFile.Get(id)
	if !ok {
		return skillsets.Set{}, fmt.Errorf("Skill Set not found")
	}
	return set, nil
}

func (s *Service) validateSkillSetMembersLocked(names []string, allowedMissing map[string]bool) ([]string, error) {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if !s.hasManagedSkillLocked(name) && !allowedMissing[name] {
			return nil, fmt.Errorf("skill %q is not a current toggleable skill", name)
		}
		normalized = append(normalized, name)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("select at least one skill")
	}
	sort.Strings(normalized)
	return normalized, nil
}

func (s *Service) hasManagedSkillLocked(name string) bool {
	for _, row := range s.rows {
		if row.Name != name {
			continue
		}
		for _, cell := range []*model.ToolSkill{row.Claude, row.Codex, row.Muse, row.Grok} {
			if cell != nil && !cell.ReadOnly && (cell.State == model.SkillStateOn || cell.State == model.SkillStateOff) {
				return true
			}
		}
	}
	return false
}

func (s *Service) skillSetRowsLocked(set skillsets.Set) []model.SkillRow {
	byName := make(map[string]model.SkillRow, len(s.rows))
	for _, row := range s.rows {
		byName[row.Name] = row
	}
	rows := make([]model.SkillRow, 0, len(set.Skills))
	for _, name := range set.Skills {
		row, ok := byName[name]
		if !ok {
			row = model.SkillRow{Name: name}
		}
		rows = append(rows, row)
	}
	return rows
}

func skillSetDirection(rows []model.SkillRow, tools []model.Tool, pending staging.Memory) (string, int) {
	eligible := 0
	allOn := true
	for _, row := range rows {
		for _, tool := range tools {
			cell := staging.SkillForTool(row, tool)
			if cell == nil || cell.ReadOnly || cell.Conflict != nil || (cell.State != model.SkillStateOn && cell.State != model.SkillStateOff) {
				continue
			}
			eligible++
			operation, _ := pending.Get(staging.Key{Tool: tool, SkillName: row.Name})
			if staging.EffectiveState(cell, operation) != model.SkillStateOn {
				allOn = false
			}
		}
	}
	if eligible == 0 {
		return "none", 0
	}
	if allOn {
		return model.OperationDisable.String(), eligible
	}
	return model.OperationEnable.String(), eligible
}

func toolStrings(tools []model.Tool) []string {
	result := make([]string, 0, len(tools))
	for _, tool := range tools {
		result = append(result, tool.String())
	}
	return result
}

func (s *Service) skillSetMutationResultLocked(message string) SkillSetMutationResult {
	return SkillSetMutationResult{Message: message, SkillSets: s.projectSkillSetsLocked(), Warning: s.skillSetsWarning}
}

func (s *Service) projectSkillSetsLocked() []SkillSet {
	result := make([]SkillSet, 0, len(s.skillSetFile.Sets))
	for _, set := range s.skillSetFile.Sets {
		result = append(result, s.projectSkillSetLocked(set))
	}
	return result
}

func (s *Service) projectSkillSetLocked(set skillsets.Set) SkillSet {
	rows := s.skillSetRowsLocked(set)
	projected := SkillSet{
		SetID: set.ID, Name: set.Name, Description: set.Description,
		Members: make([]SkillSetMember, 0, len(rows)), CreatedAt: formatTime(set.CreatedAt), UpdatedAt: formatTime(set.UpdatedAt),
	}
	for _, row := range rows {
		member := SkillSetMember{Name: row.Name, Description: row.Description, Group: normalizedGroup(row.Group).String(), Source: row.Source.String()}
		member.Claude = projectSkillSetMemberCell(row.Claude, model.ToolClaude, s.pending)
		member.Codex = projectSkillSetMemberCell(row.Codex, model.ToolCodex, s.pending)
		member.Muse = projectSkillSetMemberCell(row.Muse, model.ToolMuse, s.pending)
		member.Grok = projectSkillSetMemberCell(row.Grok, model.ToolGrok, s.pending)
		member.Available = member.Claude.Eligible || member.Codex.Eligible || member.Muse.Eligible || member.Grok.Eligible ||
			member.Claude.State == model.SkillStateConflict.String() ||
			member.Codex.State == model.SkillStateConflict.String() ||
			member.Muse.State == model.SkillStateConflict.String() ||
			member.Grok.State == model.SkillStateConflict.String()
		if !member.Available {
			projected.Unavailable++
		}
		if member.Claude.Pending != "" {
			projected.Pending++
		}
		if member.Codex.Pending != "" {
			projected.Pending++
		}
		if member.Muse.Pending != "" {
			projected.Pending++
		}
		if member.Grok.Pending != "" {
			projected.Pending++
		}
		projected.Members = append(projected.Members, member)
	}
	projected.Claude = summarizeSkillSetTool(projected.Members, model.ToolClaude)
	projected.Codex = summarizeSkillSetTool(projected.Members, model.ToolCodex)
	projected.Muse = summarizeSkillSetTool(projected.Members, model.ToolMuse)
	projected.Grok = summarizeSkillSetTool(projected.Members, model.ToolGrok)
	return projected
}

func projectSkillSetMemberCell(cell *model.ToolSkill, tool model.Tool, pending staging.Memory) SkillSetMemberCell {
	projected := SkillSetMemberCell{Tool: tool.String(), State: model.SkillStateMissing.String(), EffectiveState: model.SkillStateMissing.String(), Reason: "Not installed for this tool."}
	if cell == nil {
		return projected
	}
	projected.State = cell.State.String()
	projected.EffectiveState = cell.State.String()
	if cell.Conflict != nil || cell.State == model.SkillStateConflict {
		projected.State = model.SkillStateConflict.String()
		projected.EffectiveState = model.SkillStateConflict.String()
		projected.Reason = "Restore is blocked."
		if cell.Conflict != nil {
			projected.Reason = cell.Conflict.Message
		}
		return projected
	}
	if cell.ReadOnly || cell.State == model.SkillStateReadOnly {
		projected.State = model.SkillStateReadOnly.String()
		projected.EffectiveState = model.SkillStateReadOnly.String()
		projected.Reason = "Read-only source."
		return projected
	}
	if cell.State != model.SkillStateOn && cell.State != model.SkillStateOff {
		return projected
	}
	operation, _ := pending.Get(staging.Key{Tool: tool, SkillName: cell.Name})
	projected.Pending = operation.String()
	projected.EffectiveState = staging.EffectiveState(cell, operation).String()
	projected.Eligible = true
	projected.Reason = ""
	return projected
}

func summarizeSkillSetTool(members []SkillSetMember, tool model.Tool) SkillSetToolSummary {
	summary := SkillSetToolSummary{Tool: tool.String()}
	for _, member := range members {
		cell := member.Claude
		switch tool {
		case model.ToolCodex:
			cell = member.Codex
		case model.ToolMuse:
			cell = member.Muse
		case model.ToolGrok:
			cell = member.Grok
		}
		switch cell.State {
		case model.SkillStateOn.String():
			summary.Eligible++
			summary.On++
		case model.SkillStateOff.String():
			summary.Eligible++
			summary.Off++
		case model.SkillStateConflict.String():
			summary.Conflict++
		case model.SkillStateReadOnly.String():
			summary.ReadOnly++
		default:
			summary.Missing++
		}
		if cell.Eligible {
			if cell.EffectiveState == model.SkillStateOn.String() {
				summary.EffectiveOn++
			} else if cell.EffectiveState == model.SkillStateOff.String() {
				summary.EffectiveOff++
			}
		}
		if cell.Pending != "" {
			summary.Pending++
		}
	}
	summary.AppliedStatus = skillSetToolStatus(summary.Eligible, summary.On, summary.Off, summary.Conflict)
	summary.EffectiveStatus = skillSetToolStatus(summary.Eligible, summary.EffectiveOn, summary.EffectiveOff, summary.Conflict)
	return summary
}

func skillSetToolStatus(eligible, on, off, conflicts int) string {
	if conflicts > 0 {
		return skillSetStatusNeedsAttention
	}
	if eligible == 0 {
		return skillSetStatusUnavailable
	}
	if on == eligible {
		return skillSetStatusEnabled
	}
	if off == eligible {
		return skillSetStatusDisabled
	}
	return skillSetStatusMixed
}

func (s *Service) sourceSkillSetImpacts(installed []state.InstalledSkillEntry) ([]SkillSetImpact, string) {
	file, err := s.skillSetStore.Load()
	if err != nil {
		return []SkillSetImpact{}, err.Error()
	}
	sourceSkills := make(map[string]bool, len(installed))
	for _, skill := range installed {
		sourceSkills[skill.Name] = true
	}
	impacts := make([]SkillSetImpact, 0)
	for _, set := range file.Sets {
		matches := make([]string, 0)
		for _, name := range set.Skills {
			if sourceSkills[name] {
				matches = append(matches, name)
			}
		}
		if len(matches) > 0 {
			impacts = append(impacts, SkillSetImpact{SetID: set.ID, Name: set.Name, Skills: matches})
		}
	}
	return impacts, ""
}
