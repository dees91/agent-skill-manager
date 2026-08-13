// Package staging owns the pure pending-change behavior shared by interactive
// interfaces. It never reads or mutates the filesystem.
package staging

import (
	"github.com/dees91/agent-skill-manager/internal/model"
)

// Key identifies one tool-specific skill cell.
type Key struct {
	Tool      model.Tool
	SkillName string
}

// Store is the minimal pending-change storage used by the staging algorithms.
type Store interface {
	Get(Key) (model.OperationKind, bool)
	Set(Key, model.OperationKind)
	Delete(Key)
}

// Memory is an in-memory Store suitable for a single UI process.
type Memory map[Key]model.OperationKind

// Get implements Store.
func (m Memory) Get(key Key) (model.OperationKind, bool) {
	kind, ok := m[key]
	return kind, ok
}

// Set implements Store.
func (m Memory) Set(key Key, kind model.OperationKind) {
	m[key] = kind
}

// Delete implements Store.
func (m Memory) Delete(key Key) {
	delete(m, key)
}

// ToggleResult describes a single-cell staging update.
type ToggleResult struct {
	Changed bool
	Removed bool
}

// BatchResult describes a smart batch staging update and skipped cells.
type BatchResult struct {
	Changed         int
	Removed         int
	SkippedReadOnly int
	SkippedMissing  int
	SkippedConflict int
}

type batchCell struct {
	key            Key
	actualState    model.SkillState
	effectiveState model.SkillState
}

// ToggleCell adds or removes the natural pending operation for one cell.
func ToggleCell(store Store, row model.SkillRow, tool model.Tool) ToggleResult {
	key := Key{Tool: tool, SkillName: row.Name}
	if _, ok := store.Get(key); ok {
		store.Delete(key)
		return ToggleResult{Changed: true, Removed: true}
	}

	kind, ok := OperationForCell(SkillForTool(row, tool))
	if !ok {
		return ToggleResult{}
	}
	store.Set(key, kind)
	return ToggleResult{Changed: true}
}

// ToggleBatch smart-toggles a row scope across the provided tool columns.
func ToggleBatch(store Store, rows []model.SkillRow, tools []model.Tool) BatchResult {
	result := BatchResult{}
	cells := make([]batchCell, 0, len(rows)*len(tools))

	for _, row := range rows {
		for _, tool := range tools {
			cell, ok := batchCellForSkill(store, SkillForTool(row, tool), Key{Tool: tool, SkillName: row.Name}, &result)
			if ok {
				cells = append(cells, cell)
			}
		}
	}
	if len(cells) == 0 {
		return result
	}

	targetOperation := model.OperationDisable
	targetState := model.SkillStateOff
	for _, cell := range cells {
		if cell.effectiveState != model.SkillStateOn {
			targetOperation = model.OperationEnable
			targetState = model.SkillStateOn
			break
		}
	}

	targetCells := make([]batchCell, 0, len(cells))
	for _, cell := range cells {
		switch targetOperation {
		case model.OperationDisable:
			if cell.effectiveState == model.SkillStateOn {
				targetCells = append(targetCells, cell)
			}
		case model.OperationEnable:
			if cell.effectiveState == model.SkillStateOff {
				targetCells = append(targetCells, cell)
			}
		}
	}

	for _, cell := range targetCells {
		desiredPending := PendingForDesiredState(cell.actualState, targetState)
		current, hasCurrent := store.Get(cell.key)
		if desiredPending == "" {
			if hasCurrent {
				store.Delete(cell.key)
				result.Removed++
			}
			continue
		}
		if !hasCurrent || current != desiredPending {
			store.Set(cell.key, desiredPending)
			result.Changed++
		}
	}
	return result
}

// PendingForDesiredState returns the operation required to reach desired.
func PendingForDesiredState(actual, desired model.SkillState) model.OperationKind {
	switch {
	case actual == model.SkillStateOn && desired == model.SkillStateOff:
		return model.OperationDisable
	case actual == model.SkillStateOff && desired == model.SkillStateOn:
		return model.OperationEnable
	default:
		return ""
	}
}

// EffectiveState projects a pending operation over a scanned cell state.
func EffectiveState(skill *model.ToolSkill, pending model.OperationKind) model.SkillState {
	if skill == nil {
		return model.SkillStateMissing
	}
	switch pending {
	case model.OperationDisable:
		return model.SkillStateOff
	case model.OperationEnable:
		return model.SkillStateOn
	default:
		return skill.State
	}
}

// SkillForTool returns the row cell for one supported tool.
func SkillForTool(row model.SkillRow, tool model.Tool) *model.ToolSkill {
	switch tool {
	case model.ToolClaude:
		return row.Claude
	case model.ToolCodex:
		return row.Codex
	default:
		return nil
	}
}

// OperationForCell returns the natural toggle operation for a scanned cell.
func OperationForCell(skill *model.ToolSkill) (model.OperationKind, bool) {
	if skill == nil || skill.ReadOnly || skill.Conflict != nil {
		return "", false
	}
	switch skill.State {
	case model.SkillStateOn:
		return model.OperationDisable, true
	case model.SkillStateOff:
		return model.OperationEnable, true
	default:
		return "", false
	}
}

func batchCellForSkill(store Store, skill *model.ToolSkill, key Key, result *BatchResult) (batchCell, bool) {
	if skill == nil {
		result.SkippedMissing++
		return batchCell{}, false
	}
	if skill.State == model.SkillStateConflict || skill.Conflict != nil {
		result.SkippedConflict++
		return batchCell{}, false
	}
	if skill.ReadOnly || skill.State == model.SkillStateReadOnly {
		result.SkippedReadOnly++
		return batchCell{}, false
	}
	if skill.State != model.SkillStateOn && skill.State != model.SkillStateOff {
		result.SkippedMissing++
		return batchCell{}, false
	}

	pending, _ := store.Get(key)
	return batchCell{
		key:            key,
		actualState:    skill.State,
		effectiveState: EffectiveState(skill, pending),
	}, true
}
