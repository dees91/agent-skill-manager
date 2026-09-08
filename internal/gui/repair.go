package gui

import (
	"errors"
	"fmt"
	"sort"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// Source health states projected to the desktop interface.
const (
	SourceHealthOK          = "ok"
	SourceHealthNeedsRepair = "needs-repair"
	SourceHealthBlocked     = "blocked"
)

// InspectSources runs the explicit read-only diagnosis for every managed Git
// repository. It never fetches, mutates Git refs, or changes state. Health is
// not part of the shared snapshot because enumerating ignored worktree content
// across every checkout is too expensive for routine refreshes.
func (s *Service) InspectSources() ([]SourceHealth, error) {
	if s.SourceBusy() {
		return nil, fmt.Errorf("wait for the source operation to finish before inspecting sources")
	}
	manifest, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	service := install.NewUpdateService(s.paths, s.gitRunner)
	health := make([]SourceHealth, 0, len(manifest.Repositories))
	for _, repository := range manifest.Repositories {
		health = append(health, inspectRepositoryHealth(service, repository))
	}
	sort.SliceStable(health, func(i, j int) bool { return health[i].Group < health[j].Group })
	return health, nil
}

func inspectRepositoryHealth(service *install.UpdateService, repository state.RepositoryEntry) SourceHealth {
	entry := SourceHealth{SourceID: repositorySourceID(repository), Group: repository.Group.String(), Status: SourceHealthOK}
	_, err := service.PlanLocal(repository)
	if err == nil {
		return entry
	}
	var conflict install.CheckoutConflictError
	if errors.As(err, &conflict) {
		entry.Kind = string(conflict.Kind)
		entry.Cause = conflict.Cause()
		entry.Remedy = conflict.Remedy()
		entry.Repairable = conflict.Repairable()
		if conflict.Repairable() {
			entry.Status = SourceHealthNeedsRepair
			return entry
		}
		entry.Status = SourceHealthBlocked
		return entry
	}
	entry.Status = SourceHealthBlocked
	entry.Cause = err.Error()
	entry.Remedy = "Resolve the reported conflict, then try again."
	return entry
}

// PreviewRepair validates ownership and returns the exact checkout-relative
// paths a confirmed repair would stage. It mutates nothing.
func (s *Service) PreviewRepair(sourceID string) (RepairPreview, error) {
	var preview RepairPreview
	err := s.runSourceOperation("repair", "", func() error {
		manifest, err := s.store.Load()
		if err != nil {
			return err
		}
		repository, ok := findRepositoryByID(manifest, sourceID)
		if !ok {
			return fmt.Errorf("managed Git source not found")
		}
		plan, err := install.NewRepairService(s.paths, s.gitRunner).Plan(repository)
		if err != nil {
			return err
		}
		preview = projectRepairPreview(sourceID, repository, plan)
		return nil
	})
	if err != nil {
		return RepairPreview{}, err
	}
	return preview, nil
}

// RepairSource stages every blocking worktree path through the trash directory
// and restores tracked paths from HEAD, then returns a fresh snapshot.
func (s *Service) RepairSource(sourceID string, includeReadOnly bool) SourceMutationResult {
	result := SourceMutationResult{Completed: []SourceMutationItem{}}
	refreshed := false
	err := s.runSourceOperation("repair", "", func() error {
		defer func() {
			s.refreshSourceResult(&result, includeReadOnly)
			refreshed = true
		}()
		manifest, err := s.store.Load()
		if err != nil {
			return err
		}
		repository, ok := findRepositoryByID(manifest, sourceID)
		if !ok {
			return fmt.Errorf("managed Git source not found")
		}
		group := repository.Group.String()
		s.emitProgress(SourceProgress{Operation: "repair", Phase: "stage", Group: group, Current: 1, Total: 1, Message: "Staging blocking worktree paths…"})
		repaired, applyErr := install.NewRepairService(s.paths, s.gitRunner).Apply(repository)
		if applyErr != nil {
			result.Failure = newSourceMutationFailure("repair", sourceID, group, applyErr)
			result.Failure.CleanupPending = repaired.CleanupPending
			result.Failure.RolledBack = len(repaired.RolledBack)
			return applyErr
		}
		status := "repaired"
		if len(repaired.StagedEntries) == 0 && len(repaired.RestoredPaths) == 0 {
			status = "already-clean"
		}
		result.Completed = append(result.Completed, SourceMutationItem{SourceID: sourceID, Group: group, Status: status})
		result.Message = repairMessage(status, group, len(repaired.StagedEntries))
		return nil
	})
	if err != nil {
		if result.Failure == nil {
			result.Failure = &SourceMutationFailure{Stage: "preflight", SourceID: sourceID, Message: err.Error()}
		}
		if result.Message == "" {
			result.Message = "Repair failed."
		}
	}
	if !refreshed {
		s.attachCurrentSnapshot(&result)
	}
	return result
}

func projectRepairPreview(sourceID string, repository state.RepositoryEntry, plan install.RepairPlan) RepairPreview {
	preview := RepairPreview{
		SourceID: sourceID,
		Group:    repository.Group.String(),
		Clean:    plan.Clean,
		Entries:  make([]RepairEntry, 0, len(plan.Entries)),
	}
	for _, entry := range plan.Entries {
		preview.Entries = append(preview.Entries, RepairEntry{Path: entry.RelPath, Class: entry.Class})
	}
	preview.TrackedCount = plan.Counts[install.WorktreeEntryTracked]
	preview.UntrackedCount = plan.Counts[install.WorktreeEntryUntracked]
	preview.IgnoredCount = plan.Counts[install.WorktreeEntryIgnored]
	return preview
}

func repairMessage(status, group string, staged int) string {
	if status == "already-clean" {
		return fmt.Sprintf("%s was already clean.", group)
	}
	if staged == 1 {
		return fmt.Sprintf("Repaired %s and staged 1 path.", group)
	}
	return fmt.Sprintf("Repaired %s and staged %d paths.", group, staged)
}

// newSourceMutationFailure classifies a source failure so the interface can
// explain the cause and offer repair without parsing error text.
func newSourceMutationFailure(stage, sourceID, group string, err error) *SourceMutationFailure {
	failure := &SourceMutationFailure{Stage: stage, Group: group, SourceID: sourceID, Message: err.Error()}
	var conflict install.CheckoutConflictError
	if errors.As(err, &conflict) {
		failure.Kind = string(conflict.Kind)
		failure.Cause = conflict.Cause()
		failure.Remedy = conflict.Remedy()
		failure.Repairable = conflict.Repairable()
	}
	return failure
}
