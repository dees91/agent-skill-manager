package gui

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/state"
)

const (
	sourceKindGit   = "git"
	sourceKindLocal = "local"
)

type installDraftState struct {
	Kind         string
	Identity     install.RepoIdentity
	CheckoutPath string
	Checkout     install.CheckoutResult
	LocalSource  install.LocalSource
	Discovered   []install.DiscoveredSkill
}

type installReviewState struct {
	DraftID    string
	Selections []InstallCellRequest
}

// PrepareGitInstall clones or reuses a managed checkout and returns a path-free
// selection draft. A clone remains available for a later retry if the user cancels.
func (s *Service) PrepareGitInstall(rawURL string) (InstallDraft, error) {
	var result InstallDraft
	err := s.runSourceOperation("install", "", func() error {
		s.emitProgress(SourceProgress{Operation: "install", Phase: "validate", Message: "Validating repository URL…"})
		identity, err := install.NormalizeGitURL(rawURL)
		if err != nil {
			return err
		}
		checkoutPath, err := install.CheckoutPath(s.paths, identity)
		if err != nil {
			return err
		}
		s.emitProgress(SourceProgress{Operation: "install", Phase: "checkout", Group: identity.Group.String(), Message: "Cloning or validating the managed checkout…"})
		checkout, err := install.NewCheckoutService(s.gitRunner).EnsureCheckout(identity, checkoutPath, install.CheckoutOptions{})
		if err != nil {
			return err
		}
		s.emitProgress(SourceProgress{Operation: "install", Phase: "discover", Group: identity.Group.String(), Message: "Discovering skills…"})
		discovered, err := install.DiscoverSkills(checkoutPath)
		if err != nil {
			return err
		}
		if len(discovered) == 0 {
			return fmt.Errorf("no installable skills discovered")
		}
		manifest, err := s.store.Load()
		if err != nil {
			return err
		}
		draftID, err := randomID("draft")
		if err != nil {
			return err
		}
		internal := installDraftState{Kind: sourceKindGit, Identity: identity, CheckoutPath: checkoutPath, Checkout: checkout, Discovered: discovered}
		candidates := s.projectInstallCandidates(internal, manifest)
		s.mu.Lock()
		s.drafts[draftID] = internal
		s.mu.Unlock()
		result = InstallDraft{
			DraftID: draftID, Kind: sourceKindGit, Group: identity.Group.String(), Location: identity.OriginalURL,
			Candidates: candidates, Cloned: checkout.Cloned, Reused: checkout.Reused, RetainedClone: checkout.Cloned,
		}
		return nil
	})
	return result, err
}

// PrepareLocalInstall validates a native-picker path and returns a path-free draft.
func (s *Service) PrepareLocalInstall(selectedPath string) (InstallDraft, error) {
	if strings.TrimSpace(selectedPath) == "" {
		return InstallDraft{Kind: sourceKindLocal, Cancelled: true}, nil
	}
	var result InstallDraft
	err := s.runSourceOperation("install", "", func() error {
		s.emitProgress(SourceProgress{Operation: "install", Phase: "validate", Message: "Validating local source…"})
		source, err := install.ResolveLocalSource(s.paths, s.paths.Home, selectedPath)
		if err != nil {
			return err
		}
		s.emitProgress(SourceProgress{Operation: "install", Phase: "discover", Group: source.Group.String(), Message: "Discovering skills…"})
		discovered, err := install.DiscoverLocalSkills(source)
		if err != nil {
			return err
		}
		if len(discovered) == 0 {
			return fmt.Errorf("no installable skills discovered")
		}
		manifest, err := s.store.Load()
		if err != nil {
			return err
		}
		draftID, err := randomID("draft")
		if err != nil {
			return err
		}
		internal := installDraftState{Kind: sourceKindLocal, LocalSource: source, Discovered: discovered}
		candidates := s.projectInstallCandidates(internal, manifest)
		s.mu.Lock()
		s.drafts[draftID] = internal
		s.mu.Unlock()
		result = InstallDraft{DraftID: draftID, Kind: sourceKindLocal, Group: source.Group.String(), Location: source.CanonicalPath, Candidates: candidates}
		return nil
	})
	return result, err
}

// ReviewInstall re-discovers and preflights every selected exact cell.
func (s *Service) ReviewInstall(draftID string, selections []InstallCellRequest) (InstallReview, error) {
	var result InstallReview
	err := s.runSourceOperation("install", "", func() error {
		draft, err := s.getDraft(draftID)
		if err != nil {
			return err
		}
		normalized, options, err := normalizeInstallSelections(selections)
		if err != nil {
			return err
		}
		s.emitProgress(SourceProgress{Operation: "install", Phase: "preflight", Group: draftGroup(draft), Message: "Checking selected install targets…"})
		plan, localPlan, conflicts, err := s.planDraft(draft, options)
		if err != nil {
			return err
		}
		result = InstallReview{DraftID: draftID, Group: draftGroup(draft), Selections: normalized, Conflicts: conflicts}
		if len(conflicts) > 0 {
			return nil
		}
		if draft.Kind == sourceKindGit {
			result.CreateCount = len(plan.Links)
			result.AlreadyOnCount, result.AlreadyOffCount = alreadyCounts(plan.AlreadyInstalled)
		} else {
			result.CreateCount = len(localPlan.Links)
			result.AlreadyOnCount, result.AlreadyOffCount = alreadyCounts(localPlan.AlreadyInstalled)
		}
		reviewID, err := randomID("review")
		if err != nil {
			return err
		}
		result.ReviewID = reviewID
		result.Ready = true
		s.mu.Lock()
		s.reviews[reviewID] = installReviewState{DraftID: draftID, Selections: normalized}
		s.mu.Unlock()
		return nil
	})
	return result, err
}

// ApplyInstall re-discovers, replans, and applies one reviewed selection.
func (s *Service) ApplyInstall(reviewID string, includeReadOnly bool) SourceMutationResult {
	result := SourceMutationResult{Completed: []SourceMutationItem{}}
	refreshed := false
	err := s.runSourceOperation("install", "", func() error {
		defer func() {
			s.refreshSourceResult(&result, includeReadOnly)
			refreshed = true
		}()
		review, draft, err := s.getReview(reviewID)
		if err != nil {
			return err
		}
		_, options, err := normalizeInstallSelections(review.Selections)
		if err != nil {
			return err
		}
		group := draftGroup(draft)
		s.emitProgress(SourceProgress{Operation: "install", Phase: "revalidate", Group: group, Message: "Revalidating source and selected targets…"})
		plan, localPlan, conflicts, err := s.planDraft(draft, options)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return fmt.Errorf("install preflight changed: %s", conflicts[0].Reason)
		}
		s.emitProgress(SourceProgress{Operation: "install", Phase: "apply", Group: group, Message: "Creating skill links and saving ownership…"})
		item := SourceMutationItem{Group: group, Status: "installed"}
		if draft.Kind == sourceKindGit {
			applied, applyErr := install.NewApplyService(s.paths).Apply(plan, draft.Checkout.LastSeenCommit)
			result.CreatedLinks = len(applied.Created)
			result.AlreadyInstalled = len(applied.AlreadyInstalled)
			if applyErr != nil {
				result.Failure = &SourceMutationFailure{Stage: "apply", Group: group, Message: applyErr.Error(), RolledBack: len(applied.RolledBack)}
				return applyErr
			}
			item.SourceID = repositorySourceID(applied.Repository)
		} else {
			applied, applyErr := install.NewLocalApplyService(s.paths).Apply(localPlan)
			result.CreatedLinks = len(applied.Created)
			result.AlreadyInstalled = len(applied.AlreadyInstalled)
			if applyErr != nil {
				result.Failure = &SourceMutationFailure{Stage: "apply", Group: group, Message: applyErr.Error(), RolledBack: len(applied.RolledBack)}
				return applyErr
			}
			item.SourceID = localSourceID(applied.Source)
		}
		result.Completed = append(result.Completed, item)
		result.Message = fmt.Sprintf("Installed %d link(s); %d already installed.", result.CreatedLinks, result.AlreadyInstalled)
		s.mu.Lock()
		delete(s.reviews, reviewID)
		delete(s.drafts, review.DraftID)
		s.mu.Unlock()
		return nil
	})
	if err != nil {
		if result.Failure == nil {
			result.Failure = &SourceMutationFailure{Stage: "preflight", Message: err.Error()}
		}
		if result.Message == "" {
			result.Message = "Install failed."
		}
	}
	if !refreshed {
		s.attachCurrentSnapshot(&result)
	}
	return result
}

// UpdateSource safely updates one managed Git repository.
func (s *Service) UpdateSource(sourceID string, includeReadOnly bool) SourceMutationResult {
	result := SourceMutationResult{Completed: []SourceMutationItem{}}
	refreshed := false
	err := s.runSourceOperation("update", "", func() error {
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
		s.emitProgress(SourceProgress{Operation: "update", Phase: "fetch", Group: repository.Group.String(), Current: 1, Total: 1, Message: "Fetching and validating repository…"})
		updated, applyErr := install.NewUpdateService(s.paths, s.gitRunner).Apply(repository)
		if applyErr != nil {
			result.Failure = &SourceMutationFailure{Stage: "update", Group: repository.Group.String(), Message: applyErr.Error()}
			return applyErr
		}
		status := "up-to-date"
		if updated.Updated {
			status = "updated"
		}
		result.Completed = append(result.Completed, SourceMutationItem{SourceID: sourceID, Group: repository.Group.String(), Status: status, Before: updated.PreviousCommit, After: updated.CurrentCommit})
		result.Message = sourceUpdateMessage(result.Completed)
		return nil
	})
	if err != nil {
		if result.Failure == nil {
			result.Failure = &SourceMutationFailure{Stage: "preflight", Message: err.Error()}
		}
		if result.Message == "" {
			result.Message = "Update failed."
		}
	}
	if !refreshed {
		s.attachCurrentSnapshot(&result)
	}
	return result
}

// UpdateAllSources updates every managed Git repository in deterministic order.
func (s *Service) UpdateAllSources(includeReadOnly bool) SourceMutationResult {
	result := SourceMutationResult{Completed: []SourceMutationItem{}}
	refreshed := false
	err := s.runSourceOperation("update", "", func() error {
		defer func() {
			s.refreshSourceResult(&result, includeReadOnly)
			refreshed = true
		}()
		manifest, err := s.store.Load()
		if err != nil {
			return err
		}
		if len(manifest.Repositories) == 0 {
			result.Message = "No managed Git repositories recorded."
			return nil
		}
		service := install.NewUpdateService(s.paths, s.gitRunner)
		for index, repository := range manifest.Repositories {
			s.emitProgress(SourceProgress{Operation: "update", Phase: "fetch", Group: repository.Group.String(), Current: index + 1, Total: len(manifest.Repositories), Message: "Fetching and validating repository…"})
			updated, applyErr := service.Apply(repository)
			if applyErr != nil {
				result.Failure = &SourceMutationFailure{Stage: "update", Group: repository.Group.String(), Message: applyErr.Error()}
				return applyErr
			}
			status := "up-to-date"
			if updated.Updated {
				status = "updated"
			}
			result.Completed = append(result.Completed, SourceMutationItem{SourceID: repositorySourceID(repository), Group: repository.Group.String(), Status: status, Before: updated.PreviousCommit, After: updated.CurrentCommit})
		}
		result.Message = sourceUpdateMessage(result.Completed)
		return nil
	})
	if err != nil {
		if result.Failure == nil {
			result.Failure = &SourceMutationFailure{Stage: "preflight", Message: err.Error()}
		}
		result.Message = fmt.Sprintf("Update stopped after %d source(s).", len(result.Completed))
	}
	if !refreshed {
		s.attachCurrentSnapshot(&result)
	}
	return result
}

// PreviewUninstall validates ownership and returns exact removal counts.
func (s *Service) PreviewUninstall(sourceID string) (UninstallPreview, error) {
	var result UninstallPreview
	err := s.runSourceOperation("uninstall", "", func() error {
		manifest, err := s.store.Load()
		if err != nil {
			return err
		}
		if repository, ok := findRepositoryByID(manifest, sourceID); ok {
			s.emitProgress(SourceProgress{Operation: "uninstall", Phase: "preflight", Group: repository.Group.String(), Message: "Auditing repository ownership…"})
			plan, err := install.NewUninstallService(s.paths, s.gitRunner).Plan(repository)
			if err != nil {
				return err
			}
			on, off := referenceCounts(plan.References.References)
			impacts, warning := s.sourceSkillSetImpacts(repository.InstalledSkills)
			favoriteImpacts, favoriteWarning := s.sourceFavoriteImpacts(repository.InstalledSkills)
			result = UninstallPreview{SourceID: sourceID, Kind: sourceKindGit, Group: repository.Group.String(), Location: repository.CheckoutPath, ActiveLinks: on, DisabledLinks: off, RemovesCheckout: true, AffectedSkillSets: impacts, SkillSetImpactWarning: warning, AffectedFavorites: favoriteImpacts, FavoriteImpactWarning: favoriteWarning}
			return nil
		}
		if source, ok := findLocalSourceByID(manifest, sourceID); ok {
			s.emitProgress(SourceProgress{Operation: "uninstall", Phase: "preflight", Group: source.Group.String(), Message: "Auditing local source ownership…"})
			plan, err := install.NewLocalUninstallService(s.paths).Plan(source)
			if err != nil {
				return err
			}
			on, off := referenceCounts(plan.References.References)
			impacts, warning := s.sourceSkillSetImpacts(source.InstalledSkills)
			favoriteImpacts, favoriteWarning := s.sourceFavoriteImpacts(source.InstalledSkills)
			result = UninstallPreview{SourceID: sourceID, Kind: sourceKindLocal, Group: source.Group.String(), Location: source.CanonicalPath, ActiveLinks: on, DisabledLinks: off, PreservesSource: true, AffectedSkillSets: impacts, SkillSetImpactWarning: warning, AffectedFavorites: favoriteImpacts, FavoriteImpactWarning: favoriteWarning}
			return nil
		}
		return fmt.Errorf("managed source not found")
	})
	return result, err
}

// UninstallSource removes one whole source after exact group-name confirmation.
func (s *Service) UninstallSource(sourceID, confirmation string, includeReadOnly bool) SourceMutationResult {
	result := SourceMutationResult{Completed: []SourceMutationItem{}}
	refreshed := false
	err := s.runSourceOperation("uninstall", "", func() error {
		defer func() {
			s.refreshSourceResult(&result, includeReadOnly)
			refreshed = true
		}()
		manifest, err := s.store.Load()
		if err != nil {
			return err
		}
		if repository, ok := findRepositoryByID(manifest, sourceID); ok {
			if confirmation != repository.Group.String() {
				return fmt.Errorf("confirmation must exactly match %q", repository.Group)
			}
			s.emitProgress(SourceProgress{Operation: "uninstall", Phase: "stage", Group: repository.Group.String(), Message: "Staging owned links and checkout…"})
			removed, applyErr := install.NewUninstallService(s.paths, s.gitRunner).Apply(repository)
			result.RemovedActive, result.RemovedDisabled = len(removed.RemovedActive), len(removed.RemovedDisabled)
			if applyErr != nil {
				result.Failure = &SourceMutationFailure{Stage: "uninstall", Group: repository.Group.String(), Message: applyErr.Error(), RolledBack: len(removed.RolledBack), CleanupPending: removed.CleanupPending}
				return applyErr
			}
			result.Completed = append(result.Completed, SourceMutationItem{SourceID: sourceID, Group: repository.Group.String(), Status: "uninstalled"})
			return nil
		}
		if source, ok := findLocalSourceByID(manifest, sourceID); ok {
			if confirmation != source.Group.String() {
				return fmt.Errorf("confirmation must exactly match %q", source.Group)
			}
			s.emitProgress(SourceProgress{Operation: "uninstall", Phase: "stage", Group: source.Group.String(), Message: "Staging owned skill links…"})
			removed, applyErr := install.NewLocalUninstallService(s.paths).Apply(source)
			result.RemovedActive, result.RemovedDisabled = len(removed.RemovedActive), len(removed.RemovedDisabled)
			if applyErr != nil {
				result.Failure = &SourceMutationFailure{Stage: "uninstall", Group: source.Group.String(), Message: applyErr.Error(), RolledBack: len(removed.RolledBack), CleanupPending: removed.CleanupPending}
				return applyErr
			}
			result.Completed = append(result.Completed, SourceMutationItem{SourceID: sourceID, Group: source.Group.String(), Status: "uninstalled"})
			return nil
		}
		return fmt.Errorf("managed source not found")
	})
	if err != nil {
		if result.Failure == nil {
			result.Failure = &SourceMutationFailure{Stage: "preflight", Message: err.Error()}
		}
		if result.Message == "" {
			result.Message = "Uninstall failed."
		}
	} else if err == nil {
		result.Message = fmt.Sprintf("Uninstalled source and removed %d active, %d disabled link(s).", result.RemovedActive, result.RemovedDisabled)
	}
	if !refreshed {
		s.attachCurrentSnapshot(&result)
	}
	return result
}

func (s *Service) runSourceOperation(operation, group string, action func() error) error {
	if !s.sourceBusy.CompareAndSwap(false, true) {
		return fmt.Errorf("a source operation is already in progress")
	}
	defer s.sourceBusy.Store(false)
	s.sourceOperation.Lock()
	defer s.sourceOperation.Unlock()
	s.mu.Lock()
	pending := len(s.pending)
	s.mu.Unlock()
	if pending > 0 {
		return fmt.Errorf("apply or clear pending skill changes before managing sources")
	}
	return action()
}

func (s *Service) emitProgress(progress SourceProgress) {
	s.progressMu.Lock()
	handler := s.progress
	s.progressMu.Unlock()
	if handler != nil {
		handler(progress)
	}
}

func (s *Service) projectInstallCandidates(draft installDraftState, manifest state.Manifest) []InstallCandidate {
	candidates := make([]InstallCandidate, 0, len(draft.Discovered))
	for _, skill := range draft.Discovered {
		candidate := InstallCandidate{Name: skill.Name, RelativePath: skill.RelativePath}
		candidate.Claude = s.projectCandidateCell(draft, manifest, skill, model.ToolClaude)
		candidate.Codex = s.projectCandidateCell(draft, manifest, skill, model.ToolCodex)
		candidate.Muse = s.projectCandidateCell(draft, manifest, skill, model.ToolMuse)
		candidates = append(candidates, candidate)
	}
	return candidates
}

func (s *Service) projectCandidateCell(draft installDraftState, manifest state.Manifest, skill install.DiscoveredSkill, tool model.Tool) InstallCandidateCell {
	options := install.PlanOptions{Cells: []install.InstallCell{{SkillName: skill.Name, Tool: tool}}}
	plan, localPlan, conflicts, err := s.planKnownDraftWithManifest(draft, manifest, options)
	cell := InstallCandidateCell{Tool: tool.String(), Status: "available"}
	if err != nil {
		cell.Status, cell.Message = "conflict", err.Error()
		return cell
	}
	if len(conflicts) > 0 {
		cell.Status, cell.Message = "conflict", conflicts[0].Reason
		return cell
	}
	var already []install.AlreadyInstalled
	if draft.Kind == sourceKindGit {
		already = plan.AlreadyInstalled
	} else {
		already = localPlan.AlreadyInstalled
	}
	if len(already) == 1 {
		if already[0].State == model.SkillStateOff {
			cell.Status = "already-off"
		} else {
			cell.Status = "already-on"
		}
	}
	return cell
}

func (s *Service) planKnownDraftWithManifest(draft installDraftState, manifest state.Manifest, options install.PlanOptions) (install.InstallPlan, install.LocalInstallPlan, []InstallConflict, error) {
	var plan install.InstallPlan
	var localPlan install.LocalInstallPlan
	var planErr error
	if draft.Kind == sourceKindGit {
		plan, planErr = install.PlanInstall(s.paths, manifest, draft.Identity, draft.CheckoutPath, draft.Discovered, options)
	} else {
		localPlan, planErr = install.PlanLocalInstall(s.paths, manifest, draft.LocalSource, draft.Discovered, options)
	}
	return convertPlanError(plan, localPlan, planErr)
}

func (s *Service) planDraft(draft installDraftState, options install.PlanOptions) (install.InstallPlan, install.LocalInstallPlan, []InstallConflict, error) {
	manifest, err := s.store.Load()
	if err != nil {
		return install.InstallPlan{}, install.LocalInstallPlan{}, nil, err
	}
	return s.planDraftWithManifest(draft, manifest, options)
}

func (s *Service) planDraftWithManifest(draft installDraftState, manifest state.Manifest, options install.PlanOptions) (install.InstallPlan, install.LocalInstallPlan, []InstallConflict, error) {
	var planErr error
	var plan install.InstallPlan
	var localPlan install.LocalInstallPlan
	if draft.Kind == sourceKindGit {
		discovered, err := install.DiscoverSkills(draft.CheckoutPath)
		if err != nil {
			return plan, localPlan, nil, err
		}
		plan, planErr = install.PlanInstall(s.paths, manifest, draft.Identity, draft.CheckoutPath, discovered, options)
	} else {
		resolved, err := install.ResolveLocalSource(s.paths, s.paths.Home, draft.LocalSource.CanonicalPath)
		if err != nil {
			return plan, localPlan, nil, err
		}
		if resolved.CanonicalPath != draft.LocalSource.CanonicalPath {
			return plan, localPlan, nil, fmt.Errorf("local source identity changed")
		}
		discovered, err := install.DiscoverLocalSkills(resolved)
		if err != nil {
			return plan, localPlan, nil, err
		}
		localPlan, planErr = install.PlanLocalInstall(s.paths, manifest, resolved, discovered, options)
	}
	return convertPlanError(plan, localPlan, planErr)
}

func convertPlanError(plan install.InstallPlan, localPlan install.LocalInstallPlan, planErr error) (install.InstallPlan, install.LocalInstallPlan, []InstallConflict, error) {
	if planErr == nil {
		return plan, localPlan, nil, nil
	}
	var typed install.PlanError
	if !errors.As(planErr, &typed) {
		return plan, localPlan, nil, planErr
	}
	conflicts := make([]InstallConflict, 0, len(typed.Conflicts)+len(typed.MissingSkills))
	for _, conflict := range typed.Conflicts {
		conflicts = append(conflicts, InstallConflict{SkillName: conflict.SkillName, Tool: conflict.Tool.String(), Reason: conflict.Reason, Path: conflict.TargetPath})
	}
	for _, name := range typed.MissingSkills {
		conflicts = append(conflicts, InstallConflict{SkillName: name, Reason: "skill is no longer present in the source"})
	}
	return plan, localPlan, conflicts, nil
}

func (s *Service) getDraft(id string) (installDraftState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, ok := s.drafts[strings.TrimSpace(id)]
	if !ok {
		return installDraftState{}, fmt.Errorf("install draft is missing or expired")
	}
	return draft, nil
}

func (s *Service) getReview(id string) (installReviewState, installDraftState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	review, ok := s.reviews[strings.TrimSpace(id)]
	if !ok {
		return installReviewState{}, installDraftState{}, fmt.Errorf("install review is missing or expired")
	}
	draft, ok := s.drafts[review.DraftID]
	if !ok {
		return installReviewState{}, installDraftState{}, fmt.Errorf("install draft is missing or expired")
	}
	return review, draft, nil
}

func normalizeInstallSelections(selections []InstallCellRequest) ([]InstallCellRequest, install.PlanOptions, error) {
	if len(selections) == 0 {
		return nil, install.PlanOptions{}, fmt.Errorf("select at least one skill target")
	}
	seen := map[string]bool{}
	normalized := make([]InstallCellRequest, 0, len(selections))
	cells := make([]install.InstallCell, 0, len(selections))
	for _, selection := range selections {
		name := strings.TrimSpace(selection.SkillName)
		tool, ok := model.ParseTool(strings.ToLower(strings.TrimSpace(selection.Tool)))
		if name == "" || !ok {
			return nil, install.PlanOptions{}, fmt.Errorf("invalid install selection")
		}
		key := tool.String() + "\x00" + name
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, InstallCellRequest{SkillName: name, Tool: tool.String()})
		cells = append(cells, install.InstallCell{SkillName: name, Tool: tool})
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].SkillName != normalized[j].SkillName {
			return normalized[i].SkillName < normalized[j].SkillName
		}
		return normalized[i].Tool < normalized[j].Tool
	})
	return normalized, install.PlanOptions{Cells: cells}, nil
}

func (s *Service) refreshSourceResult(result *SourceMutationResult, includeReadOnly bool) {
	s.emitProgress(SourceProgress{Operation: "refresh", Phase: "rescan", Message: "Refreshing skills and managed sources…"})
	s.mu.Lock()
	err := s.reloadLocked(includeReadOnly)
	result.Snapshot = s.snapshotLocked()
	s.mu.Unlock()
	if err != nil {
		if result.Failure == nil {
			result.Failure = &SourceMutationFailure{Stage: "rescan", Message: err.Error()}
			result.Message = "The operation finished, but the follow-up scan failed."
		} else {
			result.Message += " The follow-up scan also failed: " + err.Error()
		}
	}
}

func (s *Service) attachCurrentSnapshot(result *SourceMutationResult) {
	s.mu.Lock()
	result.Snapshot = s.snapshotLocked()
	s.mu.Unlock()
}

func projectManagedSources(manifest state.Manifest) []ManagedSource {
	result := make([]ManagedSource, 0, len(manifest.Repositories)+len(manifest.LocalSources))
	for _, repository := range manifest.Repositories {
		claude, codex, muse := installedToolCounts(repository.InstalledSkills)
		location := repository.OriginalURL
		if location == "" {
			location = repository.CanonicalURL
		}
		result = append(result, ManagedSource{SourceID: repositorySourceID(repository), Kind: sourceKindGit, Group: repository.Group.String(), Location: location, SkillCount: len(repository.InstalledSkills), ClaudeCount: claude, CodexCount: codex, MuseCount: muse, InstalledAt: formatTime(repository.InstalledAt), Commit: repository.LastSeenCommit, CanUpdate: true, UpdateMode: "Managed Git", UpdateHint: "Use Update to fetch changes."})
	}
	for _, source := range manifest.LocalSources {
		claude, codex, muse := installedToolCounts(source.InstalledSkills)
		result = append(result, ManagedSource{SourceID: localSourceID(source), Kind: sourceKindLocal, Group: source.Group.String(), Location: source.CanonicalPath, SkillCount: len(source.InstalledSkills), ClaudeCount: claude, CodexCount: codex, MuseCount: muse, InstalledAt: formatTime(source.InstalledAt), CanUpdate: false, UpdateMode: "Linked folder", UpdateHint: "Changes are read directly; no update needed."})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Group != result[j].Group {
			return result[i].Group < result[j].Group
		}
		return result[i].Kind < result[j].Kind
	})
	return result
}

func installedToolCounts(skills []state.InstalledSkillEntry) (int, int, int) {
	claude, codex, muse := 0, 0, 0
	for _, skill := range skills {
		for _, tool := range skill.Tools {
			switch tool {
			case model.ToolClaude:
				claude++
			case model.ToolCodex:
				codex++
			case model.ToolMuse:
				muse++
			}
		}
	}
	return claude, codex, muse
}

func repositorySourceID(repository state.RepositoryEntry) string {
	return opaqueSourceID(sourceKindGit, repository.Host+"/"+repository.RepoPath)
}

func localSourceID(source state.LocalSourceEntry) string {
	return opaqueSourceID(sourceKindLocal, source.CanonicalPath)
}

func opaqueSourceID(kind, identity string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + identity))
	return kind + ":" + hex.EncodeToString(sum[:16])
}

func findRepositoryByID(manifest state.Manifest, id string) (state.RepositoryEntry, bool) {
	for _, repository := range manifest.Repositories {
		if repositorySourceID(repository) == id {
			return repository, true
		}
	}
	return state.RepositoryEntry{}, false
}

func findLocalSourceByID(manifest state.Manifest, id string) (state.LocalSourceEntry, bool) {
	for _, source := range manifest.LocalSources {
		if localSourceID(source) == id {
			return source, true
		}
	}
	return state.LocalSourceEntry{}, false
}

func randomID(prefix string) (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("create session identifier: %w", err)
	}
	return prefix + ":" + hex.EncodeToString(buffer), nil
}

func draftGroup(draft installDraftState) string {
	if draft.Kind == sourceKindGit {
		return draft.Identity.Group.String()
	}
	return draft.LocalSource.Group.String()
}

func alreadyCounts(items []install.AlreadyInstalled) (int, int) {
	on, off := 0, 0
	for _, item := range items {
		if item.State == model.SkillStateOff {
			off++
		} else {
			on++
		}
	}
	return on, off
}

func referenceCounts(references []install.RepositoryReference) (int, int) {
	on, off := 0, 0
	for _, reference := range references {
		if reference.State == model.SkillStateOff {
			off++
		} else {
			on++
		}
	}
	return on, off
}

func sourceUpdateMessage(items []SourceMutationItem) string {
	updated, current := 0, 0
	for _, item := range items {
		if item.Status == "updated" {
			updated++
		} else {
			current++
		}
	}
	return fmt.Sprintf("Updated %d source(s); %d already up to date.", updated, current)
}
