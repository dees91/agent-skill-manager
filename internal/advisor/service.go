package advisor

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/ops"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/scan"
)

const (
	ActionEnable     = "enable"
	ActionShare      = "share"
	ActionAlreadyOn  = "already_on"
	ActionDisable    = "disable"
	ActionRelease    = "release"
	ActionAlreadyOff = "already_off"
)

// Service coordinates advisor receipts with the existing reversible toggle domain.
type Service struct {
	paths paths.Paths
	store store
	now   func() time.Time
	newID func() (string, error)
}

// New creates an advisor service for the provided paths.
func New(p paths.Paths) *Service {
	return &Service{paths: p, store: newStore(p), now: time.Now, newID: randomID}
}

type activationPlan struct {
	action Action
	op     *model.PlannedOperation
	lease  *lease
}

// Activate temporarily enables or shares up to five skills for one tool.
func (s *Service) Activate(tool model.Tool, skillNames []string, dryRun bool) (result ActivateResult, err error) {
	result = ActivateResult{APIVersion: APIVersion, DryRun: dryRun, Tool: tool, Actions: []Action{}}
	names, err := validateActivationInput(tool, skillNames)
	if err != nil {
		return result, err
	}
	lock, err := s.store.lock(!dryRun)
	if err != nil {
		return result, err
	}
	defer func() {
		if closeErr := lock.close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	contents, err := s.store.load()
	if err != nil {
		return result, err
	}
	plans, err := s.planActivation(contents, tool, names)
	if err != nil {
		return result, err
	}
	for _, plan := range plans {
		result.Actions = append(result.Actions, plan.action)
	}
	if dryRun {
		return result, nil
	}

	claimed := 0
	for _, plan := range plans {
		if plan.action.Action != ActionAlreadyOn {
			claimed++
		}
	}
	if claimed == 0 {
		return result, nil
	}
	receiptID, err := s.uniqueReceiptID(contents)
	if err != nil {
		return result, err
	}
	result.ReceiptID = receiptID
	created := receipt{ID: receiptID, Tool: tool, CreatedAt: s.now().UTC(), Skills: []string{}}
	for _, plan := range plans {
		if plan.action.Action == ActionAlreadyOn {
			continue
		}
		created.Skills = append(created.Skills, plan.action.Skill)
		if plan.action.Action == ActionShare {
			index := contents.leaseIndex(tool, plan.action.Skill)
			contents.Leases[index].ReceiptIDs = append(contents.Leases[index].ReceiptIDs, receiptID)
			continue
		}
		newLease := *plan.lease
		newLease.ReceiptIDs = []string{receiptID}
		contents.Leases = append(contents.Leases, newLease)
	}
	contents.Receipts = append(contents.Receipts, created)
	contents.normalizeOrder()
	if err := s.store.backupExisting(); err != nil {
		return result, err
	}
	if err := s.store.save(contents); err != nil {
		return result, err
	}

	toggleService := ops.New(s.paths)
	for index, plan := range plans {
		if plan.action.Action != ActionEnable {
			continue
		}
		apply := toggleService.Apply([]model.PlannedOperation{*plan.op})
		if apply.Failed == nil {
			continue
		}
		firstUnattempted := index
		if len(apply.Completed) > 0 {
			firstUnattempted++
		}
		cleanupErr := s.removeUnattemptedEnableClaims(&contents, receiptID, tool, plans[firstUnattempted:])
		if cleanupErr == nil {
			cleanupErr = s.store.save(contents)
		}
		if cleanupErr != nil {
			return result, fmt.Errorf("%w; additionally failed to reconcile advisor receipt %s: %v", apply.Failed.Err, receiptID, cleanupErr)
		}
		if contents.receiptIndex(receiptID) < 0 {
			result.ReceiptID = ""
		}
		return result, fmt.Errorf("activate receipt %s: %w", receiptID, apply.Failed.Err)
	}
	return result, nil
}

func (s *Service) planActivation(contents file, tool model.Tool, names []string) ([]activationPlan, error) {
	cells, err := s.cellsForTool(tool)
	if err != nil {
		return nil, err
	}
	toggleService := ops.New(s.paths)
	plans := make([]activationPlan, 0, len(names))
	for _, name := range names {
		cell, ok := cells[name]
		if !ok {
			return nil, fmt.Errorf("skill %s/%s is not installed", tool, name)
		}
		if cell.State == model.SkillStateConflict {
			return nil, fmt.Errorf("skill %s/%s is in conflict", tool, name)
		}
		if cell.ReadOnly || cell.State == model.SkillStateReadOnly || cell.State == model.SkillStateMissing {
			return nil, fmt.Errorf("skill %s/%s is not toggleable", tool, name)
		}
		leaseIndex := contents.leaseIndex(tool, name)
		switch cell.State {
		case model.SkillStateOn:
			if leaseIndex < 0 {
				plans = append(plans, activationPlan{action: Action{Skill: name, Action: ActionAlreadyOn}})
				continue
			}
			currentLease := contents.Leases[leaseIndex]
			if !activeCellMatchesLease(cell, currentLease) {
				return nil, fmt.Errorf("advisor lease drift for %s/%s", tool, name)
			}
			plans = append(plans, activationPlan{action: Action{Skill: name, Action: ActionShare}})
		case model.SkillStateOff:
			if leaseIndex >= 0 {
				return nil, fmt.Errorf("advisor lease drift for %s/%s: skill is OFF while receipts remain", tool, name)
			}
			operation, err := toggleService.PlanEnable(tool, name)
			if err != nil {
				return nil, err
			}
			newLease := leaseFromEnable(operation)
			plans = append(plans, activationPlan{action: Action{Skill: name, Action: ActionEnable}, op: &operation, lease: &newLease})
		default:
			return nil, fmt.Errorf("skill %s/%s has unsupported state %s", tool, name, cell.State)
		}
	}
	return plans, nil
}

func (s *Service) removeUnattemptedEnableClaims(contents *file, receiptID string, tool model.Tool, plans []activationPlan) error {
	for _, plan := range plans {
		if plan.action.Action != ActionEnable {
			continue
		}
		if err := contents.removeClaim(receiptID, tool, plan.action.Skill); err != nil {
			return err
		}
	}
	return nil
}

// Cleanup releases one exact receipt and restores skills after the last claim.
func (s *Service) Cleanup(receiptID string, dryRun bool) (result CleanupResult, err error) {
	result = CleanupResult{APIVersion: APIVersion, DryRun: dryRun, ReceiptID: receiptID, Actions: []Action{}}
	receiptID = strings.TrimSpace(receiptID)
	result.ReceiptID = receiptID
	if !validReceiptID(receiptID) {
		return result, fmt.Errorf("invalid advisor receipt ID %q", receiptID)
	}
	lock, err := s.store.lock(!dryRun)
	if err != nil {
		return result, err
	}
	defer func() {
		if closeErr := lock.close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	contents, err := s.store.load()
	if err != nil {
		return result, err
	}
	receiptIndex := contents.receiptIndex(receiptID)
	if receiptIndex < 0 {
		return result, fmt.Errorf("advisor receipt %s not found", receiptID)
	}
	currentReceipt := contents.Receipts[receiptIndex]
	result.Tool = currentReceipt.Tool
	plans, operations, err := s.planCleanup(contents, currentReceipt)
	if err != nil {
		return result, err
	}
	for _, plan := range plans {
		result.Actions = append(result.Actions, plan)
	}
	if dryRun {
		return result, nil
	}
	if err := s.store.backupExisting(); err != nil {
		return result, err
	}
	toggleService := ops.New(s.paths)
	operationIndex := 0
	for _, action := range plans {
		if action.Action == ActionDisable {
			apply := toggleService.Apply([]model.PlannedOperation{operations[operationIndex]})
			operationIndex++
			if apply.Failed != nil {
				return result, fmt.Errorf("cleanup receipt %s: %w", receiptID, apply.Failed.Err)
			}
		}
		if err := contents.removeClaim(receiptID, currentReceipt.Tool, action.Skill); err != nil {
			return result, err
		}
		if err := s.store.save(contents); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *Service) planCleanup(contents file, currentReceipt receipt) ([]Action, []model.PlannedOperation, error) {
	cells, err := s.cellsForTool(currentReceipt.Tool)
	if err != nil {
		return nil, nil, err
	}
	actions := make([]Action, 0, len(currentReceipt.Skills))
	requests := []ops.PlanRequest{}
	lastClaimSkills := []string{}
	for _, skillName := range currentReceipt.Skills {
		leaseIndex := contents.leaseIndex(currentReceipt.Tool, skillName)
		if leaseIndex < 0 {
			return nil, nil, fmt.Errorf("advisor receipt %s has no lease for %s/%s", currentReceipt.ID, currentReceipt.Tool, skillName)
		}
		currentLease := contents.Leases[leaseIndex]
		cell, ok := cells[skillName]
		if len(currentLease.ReceiptIDs) > 1 {
			if !ok {
				return nil, nil, fmt.Errorf("advisor lease drift for %s/%s: skill is missing", currentReceipt.Tool, skillName)
			}
			switch cell.State {
			case model.SkillStateOn:
				if !activeCellMatchesLease(cell, currentLease) {
					return nil, nil, fmt.Errorf("advisor lease drift for %s/%s", currentReceipt.Tool, skillName)
				}
			case model.SkillStateOff:
				if !disabledCellMatchesLease(cell, currentLease) {
					return nil, nil, fmt.Errorf("advisor lease drift for %s/%s", currentReceipt.Tool, skillName)
				}
			case model.SkillStateConflict:
				return nil, nil, fmt.Errorf("advisor cleanup conflict for %s/%s", currentReceipt.Tool, skillName)
			default:
				return nil, nil, fmt.Errorf("advisor lease drift for %s/%s: state %s", currentReceipt.Tool, skillName, cell.State)
			}
			actions = append(actions, Action{Skill: skillName, Action: ActionRelease})
			continue
		}
		if !ok {
			return nil, nil, fmt.Errorf("advisor lease drift for %s/%s: skill is missing", currentReceipt.Tool, skillName)
		}
		switch cell.State {
		case model.SkillStateOn:
			if !activeCellMatchesLease(cell, currentLease) {
				return nil, nil, fmt.Errorf("advisor lease drift for %s/%s", currentReceipt.Tool, skillName)
			}
			actions = append(actions, Action{Skill: skillName, Action: ActionDisable})
			requests = append(requests, ops.PlanRequest{Kind: model.OperationDisable, Tool: currentReceipt.Tool, SkillName: skillName})
			lastClaimSkills = append(lastClaimSkills, skillName)
		case model.SkillStateOff:
			if !disabledCellMatchesLease(cell, currentLease) {
				return nil, nil, fmt.Errorf("advisor lease drift for %s/%s", currentReceipt.Tool, skillName)
			}
			actions = append(actions, Action{Skill: skillName, Action: ActionAlreadyOff})
		case model.SkillStateConflict:
			return nil, nil, fmt.Errorf("advisor cleanup conflict for %s/%s", currentReceipt.Tool, skillName)
		default:
			return nil, nil, fmt.Errorf("advisor lease drift for %s/%s: state %s", currentReceipt.Tool, skillName, cell.State)
		}
	}
	operations, err := ops.New(s.paths).PlanBatch(requests)
	if err != nil {
		return nil, nil, err
	}
	for index, operation := range operations {
		leaseIndex := contents.leaseIndex(currentReceipt.Tool, lastClaimSkills[index])
		if leaseIndex < 0 || !disableOperationMatchesLease(operation, contents.Leases[leaseIndex]) {
			return nil, nil, fmt.Errorf("advisor lease drift for %s/%s", currentReceipt.Tool, lastClaimSkills[index])
		}
	}
	return actions, operations, nil
}

// Status lists outstanding receipts without exposing filesystem paths.
func (s *Service) Status(tool *model.Tool) (result StatusResult, err error) {
	result = StatusResult{APIVersion: APIVersion, Capabilities: Capabilities(), Receipts: []ReceiptStatus{}}
	if tool != nil {
		if _, ok := model.ParseTool(tool.String()); !ok {
			return result, fmt.Errorf("unsupported tool %q", *tool)
		}
	}
	lock, err := s.store.lock(false)
	if err != nil {
		return result, err
	}
	defer func() {
		if closeErr := lock.close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	contents, err := s.store.load()
	if err != nil {
		return result, err
	}
	for _, current := range contents.Receipts {
		if tool != nil && current.Tool != *tool {
			continue
		}
		result.Receipts = append(result.Receipts, ReceiptStatus{
			ReceiptID: current.ID,
			Tool:      current.Tool,
			CreatedAt: current.CreatedAt,
			Skills:    append([]string(nil), current.Skills...),
		})
	}
	return result, nil
}

func (s *Service) cellsForTool(tool model.Tool) (map[string]model.ToolSkill, error) {
	skills, err := scan.New(s.paths).All()
	if err != nil {
		return nil, err
	}
	cells := map[string]model.ToolSkill{}
	for _, current := range skills {
		if current.Tool != tool {
			continue
		}
		existing, ok := cells[current.Name]
		if !ok || advisorCellPriority(current) > advisorCellPriority(existing) {
			cells[current.Name] = current
		}
	}
	return cells, nil
}

func advisorCellPriority(cell model.ToolSkill) int {
	if cell.State == model.SkillStateConflict {
		return 4
	}
	if cell.Toggleable() {
		return 3
	}
	if cell.ReadOnly || cell.State == model.SkillStateReadOnly {
		return 1
	}
	return 0
}

func validateActivationInput(tool model.Tool, names []string) ([]string, error) {
	if _, ok := model.ParseTool(tool.String()); !ok {
		return nil, fmt.Errorf("unsupported tool %q", tool)
	}
	if len(names) == 0 || len(names) > MaxSkillsPerActivation {
		return nil, fmt.Errorf("advisor activate requires 1-%d skills", MaxSkillsPerActivation)
	}
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		normalized = append(normalized, strings.TrimSpace(name))
	}
	if err := validateUniqueSkillNames(normalized); err != nil {
		return nil, err
	}
	sort.Strings(normalized)
	return normalized, nil
}

func leaseFromEnable(operation model.PlannedOperation) lease {
	return lease{
		Tool:          operation.Tool,
		SkillName:     operation.SkillName,
		OriginalPath:  operation.ToPath,
		DisabledPath:  operation.FromPath,
		EntryType:     operation.EntryType,
		SymlinkTarget: operation.SymlinkTarget,
		ReceiptIDs:    []string{},
	}
}

func activeCellMatchesLease(cell model.ToolSkill, current lease) bool {
	return cell.Tool == current.Tool && cell.Name == current.SkillName &&
		cell.ActivePath == current.OriginalPath && cell.EntryType == current.EntryType &&
		cell.SymlinkTarget == current.SymlinkTarget
}

func disabledCellMatchesLease(cell model.ToolSkill, current lease) bool {
	return cell.Tool == current.Tool && cell.Name == current.SkillName &&
		cell.ActivePath == current.OriginalPath && cell.DisabledPath == current.DisabledPath &&
		cell.EntryType == current.EntryType && cell.SymlinkTarget == current.SymlinkTarget
}

func disableOperationMatchesLease(operation model.PlannedOperation, current lease) bool {
	return operation.Tool == current.Tool && operation.SkillName == current.SkillName &&
		operation.FromPath == current.OriginalPath && operation.ToPath == current.DisabledPath &&
		operation.EntryType == current.EntryType && operation.SymlinkTarget == current.SymlinkTarget
}

func (s *Service) uniqueReceiptID(contents file) (string, error) {
	for attempt := 0; attempt < 8; attempt++ {
		id, err := s.newID()
		if err != nil {
			return "", fmt.Errorf("generate advisor receipt ID: %w", err)
		}
		if validReceiptID(id) && contents.receiptIndex(id) < 0 {
			return id, nil
		}
	}
	return "", fmt.Errorf("generate a unique advisor receipt ID")
}

func randomID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
