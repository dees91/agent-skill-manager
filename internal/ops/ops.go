package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/scan"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// Service plans and applies reversible skill toggle operations.
type Service struct {
	paths    paths.Paths
	store    state.Store
	scanner  scan.Scanner
	now      func() time.Time
	backedUp bool
}

// New creates an operation service for the provided paths.
func New(p paths.Paths) *Service {
	return &Service{
		paths:   p,
		store:   state.New(p),
		scanner: scan.New(p),
		now:     time.Now,
	}
}

// OperationFailure describes the first operation that failed in a batch.
type OperationFailure struct {
	Operation model.PlannedOperation
	Err       error
}

// PlanRequest identifies one requested toggle operation before paths are resolved.
type PlanRequest struct {
	Kind      model.OperationKind
	Tool      model.Tool
	SkillName string
}

// ApplyResult describes the completed prefix and first failed operation.
type ApplyResult struct {
	Completed []model.PlannedOperation
	Failed    *OperationFailure
}

// PlanDisable validates and builds a disable operation for an active skill.
func (s *Service) PlanDisable(tool model.Tool, skillName string) (model.PlannedOperation, error) {
	operations, err := s.PlanBatch([]PlanRequest{{
		Kind:      model.OperationDisable,
		Tool:      tool,
		SkillName: skillName,
	}})
	if err != nil {
		return model.PlannedOperation{}, err
	}
	if len(operations) != 1 {
		return model.PlannedOperation{}, fmt.Errorf("planned %d operations, want 1", len(operations))
	}
	return operations[0], nil
}

// PlanEnable validates and builds an enable operation from the state manifest.
func (s *Service) PlanEnable(tool model.Tool, skillName string) (model.PlannedOperation, error) {
	operations, err := s.PlanBatch([]PlanRequest{{
		Kind:      model.OperationEnable,
		Tool:      tool,
		SkillName: skillName,
	}})
	if err != nil {
		return model.PlannedOperation{}, err
	}
	if len(operations) != 1 {
		return model.PlannedOperation{}, fmt.Errorf("planned %d operations, want 1", len(operations))
	}
	return operations[0], nil
}

// PlanBatch validates and builds multiple toggle operations with shared scans.
func (s *Service) PlanBatch(requests []PlanRequest) ([]model.PlannedOperation, error) {
	if len(requests) == 0 {
		return []model.PlannedOperation{}, nil
	}

	var managedByKey map[state.EntryKey]model.ToolSkill
	var manifest state.Manifest
	loadedManifest := false

	operations := make([]model.PlannedOperation, 0, len(requests))
	for _, request := range requests {
		var (
			op  model.PlannedOperation
			err error
		)
		switch request.Kind {
		case model.OperationDisable:
			if managedByKey == nil {
				managed, scanErr := s.scanner.Managed()
				if scanErr != nil {
					return nil, scanErr
				}
				managedByKey = indexManagedSkills(managed)
			}
			op, err = s.planDisableFromManaged(request.Tool, request.SkillName, managedByKey)
		case model.OperationEnable:
			if !loadedManifest {
				var loadErr error
				manifest, loadErr = s.store.Load()
				if loadErr != nil {
					return nil, loadErr
				}
				loadedManifest = true
			}
			op, err = s.planEnableFromManifest(request.Tool, request.SkillName, manifest)
		default:
			err = fmt.Errorf("unsupported operation kind %q", request.Kind)
		}
		if err != nil {
			return nil, err
		}
		operations = append(operations, op)
	}
	return operations, nil
}

func indexManagedSkills(skills []model.ToolSkill) map[state.EntryKey]model.ToolSkill {
	index := make(map[state.EntryKey]model.ToolSkill, len(skills))
	for _, skill := range skills {
		index[state.EntryKey{Tool: skill.Tool, SkillName: skill.Name}] = skill
	}
	return index
}

func (s *Service) planDisableFromManaged(tool model.Tool, skillName string, managed map[state.EntryKey]model.ToolSkill) (model.PlannedOperation, error) {
	disabledPath, err := s.store.DisabledPath(tool, skillName)
	if err != nil {
		return model.PlannedOperation{}, err
	}

	skill, ok := managed[state.EntryKey{Tool: tool, SkillName: skillName}]
	if !ok {
		return model.PlannedOperation{}, fmt.Errorf("active skill %s/%s not found", tool, skillName)
	}
	if skill.State != model.SkillStateOn || skill.ReadOnly || skill.ActivePath == "" {
		return model.PlannedOperation{}, fmt.Errorf("%s/%s is not active and toggleable", tool, skillName)
	}

	op := model.PlannedOperation{
		Kind:          model.OperationDisable,
		Tool:          tool,
		SkillName:     skillName,
		FromPath:      skill.ActivePath,
		ToPath:        disabledPath,
		EntryType:     skill.EntryType,
		SymlinkTarget: skill.SymlinkTarget,
		Source:        skill.Source,
		Group:         skill.Group,
	}
	if err := validateDisable(op); err != nil {
		return model.PlannedOperation{}, err
	}
	return op, nil
}

func (s *Service) planEnableFromManifest(tool model.Tool, skillName string, manifest state.Manifest) (model.PlannedOperation, error) {
	expectedDisabledPath, err := s.store.DisabledPath(tool, skillName)
	if err != nil {
		return model.PlannedOperation{}, err
	}

	entry, ok := manifest.Get(tool, skillName)
	if !ok {
		return model.PlannedOperation{}, fmt.Errorf("disabled skill %s/%s not found in state", tool, skillName)
	}
	if entry.OriginalPath == "" || entry.DisabledPath == "" {
		return model.PlannedOperation{}, fmt.Errorf("disabled skill %s/%s has incomplete restore paths", tool, skillName)
	}
	if entry.DisabledPath != expectedDisabledPath {
		return model.PlannedOperation{}, fmt.Errorf("disabled skill %s/%s path %q does not match expected %q", tool, skillName, entry.DisabledPath, expectedDisabledPath)
	}

	op := model.PlannedOperation{
		Kind:          model.OperationEnable,
		Tool:          tool,
		SkillName:     skillName,
		FromPath:      entry.DisabledPath,
		ToPath:        entry.OriginalPath,
		EntryType:     entry.EntryType,
		SymlinkTarget: entry.SymlinkTarget,
		Source:        entry.Source,
		Group:         entry.Group,
	}
	if err := validateEnable(op, manifest); err != nil {
		return model.PlannedOperation{}, err
	}
	return op, nil
}

// Apply executes operations in deterministic backend order.
func (s *Service) Apply(operations []model.PlannedOperation) ApplyResult {
	ordered := SortOperationsForApply(operations)
	result := ApplyResult{Completed: []model.PlannedOperation{}}
	if len(ordered) == 0 {
		return result
	}

	if !s.backedUp {
		if _, err := s.store.BackupExisting(); err != nil {
			result.Failed = &OperationFailure{Operation: ordered[0], Err: err}
			return result
		}
		s.backedUp = true
	}

	manifest, err := s.store.Load()
	if err != nil {
		result.Failed = &OperationFailure{Operation: ordered[0], Err: err}
		return result
	}

	for _, op := range ordered {
		if err := validateForApply(op, manifest); err != nil {
			if saveErr := s.saveCompletedProgress(result.Completed, manifest); saveErr != nil {
				err = fmt.Errorf("%w; additionally failed to persist completed state: %v", err, saveErr)
			}
			result.Failed = &OperationFailure{Operation: op, Err: err}
			return result
		}

		destinationMode := os.FileMode(0o755)
		if pathInsideState(s.paths.StateDir, filepath.Dir(op.ToPath)) {
			destinationMode = 0o700
		}
		if err := os.MkdirAll(filepath.Dir(op.ToPath), destinationMode); err != nil {
			if saveErr := s.saveCompletedProgress(result.Completed, manifest); saveErr != nil {
				err = fmt.Errorf("%w; additionally failed to persist completed state: %v", err, saveErr)
			}
			result.Failed = &OperationFailure{Operation: op, Err: fmt.Errorf("create destination parent: %w", err)}
			return result
		}
		if err := os.Rename(op.FromPath, op.ToPath); err != nil {
			if saveErr := s.saveCompletedProgress(result.Completed, manifest); saveErr != nil {
				err = fmt.Errorf("%w; additionally failed to persist completed state: %v", err, saveErr)
			}
			result.Failed = &OperationFailure{Operation: op, Err: fmt.Errorf("move %s to %s: %w", op.FromPath, op.ToPath, err)}
			return result
		}

		switch op.Kind {
		case model.OperationDisable:
			manifest.Upsert(state.DisabledEntry{
				Tool:          op.Tool,
				SkillName:     op.SkillName,
				OriginalPath:  op.FromPath,
				DisabledPath:  op.ToPath,
				EntryType:     op.EntryType,
				SymlinkTarget: op.SymlinkTarget,
				Source:        op.Source,
				Group:         op.Group,
				DisabledAt:    s.now().UTC(),
			})
		case model.OperationEnable:
			manifest.Remove(op.Tool, op.SkillName)
		}

		result.Completed = append(result.Completed, op)
	}

	if err := s.saveCompletedProgress(result.Completed, manifest); err != nil {
		result.Failed = &OperationFailure{Operation: result.Completed[len(result.Completed)-1], Err: err}
		return result
	}
	return result
}

func pathInsideState(stateDir, candidate string) bool {
	relative, err := filepath.Rel(filepath.Clean(stateDir), filepath.Clean(candidate))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (s *Service) saveCompletedProgress(completed []model.PlannedOperation, manifest state.Manifest) error {
	if len(completed) == 0 {
		return nil
	}
	return s.store.Save(manifest)
}

// SortOperationsForApply returns a sorted copy using backend apply order.
func SortOperationsForApply(operations []model.PlannedOperation) []model.PlannedOperation {
	ordered := append([]model.PlannedOperation(nil), operations...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Kind != ordered[j].Kind {
			return operationRank(ordered[i].Kind) < operationRank(ordered[j].Kind)
		}
		if ordered[i].Tool != ordered[j].Tool {
			return ordered[i].Tool.String() < ordered[j].Tool.String()
		}
		return ordered[i].SkillName < ordered[j].SkillName
	})
	return ordered
}

func operationRank(kind model.OperationKind) int {
	switch kind {
	case model.OperationDisable:
		return 0
	case model.OperationEnable:
		return 1
	default:
		return 2
	}
}

func validateForApply(op model.PlannedOperation, manifest state.Manifest) error {
	switch op.Kind {
	case model.OperationDisable:
		return validateDisable(op)
	case model.OperationEnable:
		return validateEnable(op, manifest)
	default:
		return fmt.Errorf("unsupported operation kind %q", op.Kind)
	}
}

func validateDisable(op model.PlannedOperation) error {
	if op.Kind != model.OperationDisable {
		return fmt.Errorf("operation %q is not disable", op.Kind)
	}
	if op.FromPath == "" || op.ToPath == "" {
		return fmt.Errorf("disable %s/%s has empty source or destination", op.Tool, op.SkillName)
	}
	if err := validateExistingEntry(op.FromPath, op.EntryType); err != nil {
		return fmt.Errorf("validate disable source: %w", err)
	}
	if err := validatePathFree(op.ToPath); err != nil {
		return fmt.Errorf("validate disable destination: %w", err)
	}
	return nil
}

func validateEnable(op model.PlannedOperation, manifest state.Manifest) error {
	if op.Kind != model.OperationEnable {
		return fmt.Errorf("operation %q is not enable", op.Kind)
	}
	if op.FromPath == "" || op.ToPath == "" {
		return fmt.Errorf("enable %s/%s has empty source or destination", op.Tool, op.SkillName)
	}

	entry, ok := manifest.Get(op.Tool, op.SkillName)
	if !ok {
		return fmt.Errorf("disabled skill %s/%s not found in state", op.Tool, op.SkillName)
	}
	if entry.DisabledPath != op.FromPath || entry.OriginalPath != op.ToPath {
		return fmt.Errorf("enable %s/%s does not match state paths", op.Tool, op.SkillName)
	}
	if err := validateExistingEntry(op.FromPath, op.EntryType); err != nil {
		return fmt.Errorf("validate enable source: %w", err)
	}
	if err := validatePathFree(op.ToPath); err != nil {
		return fmt.Errorf("validate enable destination: %w", err)
	}
	return nil
}

func validateExistingEntry(path string, entryType model.EntryType) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	switch entryType {
	case model.EntryTypeSymlink:
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%s is not a symlink", path)
		}
	case model.EntryTypeDir:
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
	}
	return nil
}

func validatePathFree(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
