package gui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func supportedExtendTools() string {
	labels := make([]string, 0, len(model.Tools()))
	for _, tool := range model.Tools() {
		labels = append(labels, tool.String())
	}
	return strings.Join(labels, ", ")
}

// PreviewExtend validates the tool and returns the read-only impact of
// linking every recorded skill to it. It rejects pending toggles and
// concurrent source operations like every other source mutation entry point.
func (s *Service) PreviewExtend(toolName string) (ExtendPreview, error) {
	var result ExtendPreview
	err := s.runSourceOperation("extend", "", func() error {
		tool, ok := model.ParseTool(toolName)
		if !ok {
			return fmt.Errorf("unknown tool %q (supported: %s)", toolName, supportedExtendTools())
		}
		s.emitProgress(SourceProgress{Operation: "extend", Phase: "preflight", Message: "Planning recorded skill links…"})
		manifest, err := s.store.Load()
		if err != nil {
			return err
		}
		plan, err := install.PlanExtend(s.paths, manifest, tool)
		if err != nil {
			return err
		}
		result = projectExtendPreview(plan)
		return nil
	})
	return result, err
}

// ExtendSources links every recorded skill to one tool in manifest order and
// stops at the first failed source. Like every other source mutation it
// returns only the result: failures travel in Failure with a fresh snapshot,
// so the frontend always sees the completed prefix.
func (s *Service) ExtendSources(toolName string, includeReadOnly bool) SourceMutationResult {
	result := SourceMutationResult{Completed: []SourceMutationItem{}}
	tool, ok := model.ParseTool(toolName)
	if !ok {
		result.Failure = &SourceMutationFailure{Stage: "preflight", Message: fmt.Sprintf("unknown tool %q (supported: %s)", toolName, supportedExtendTools())}
		result.Message = "Extend failed."
		s.attachCurrentSnapshot(&result)
		return result
	}
	refreshed := false
	err := s.runSourceOperation("extend", "", func() error {
		defer func() {
			s.refreshSourceResult(&result, includeReadOnly)
			refreshed = true
		}()
		s.emitProgress(SourceProgress{Operation: "extend", Phase: "start", Message: fmt.Sprintf("Extending recorded sources to %s…", tool)})
		manifest, err := s.store.Load()
		if err != nil {
			return err
		}
		applied, applyErr := install.NewExtendService(s.paths).Apply(tool, func(progress install.ExtendProgress) {
			s.emitProgress(SourceProgress{Operation: "extend", Phase: "link", Group: progress.Group.String(), Current: progress.Current, Total: progress.Total, Message: "Linking recorded skills…"})
		})
		disabled := 0
		for _, done := range applied.Completed {
			result.CreatedLinks += done.Created
			result.AlreadyInstalled += done.AlreadyInstalled
			disabled += done.Disabled
			result.Completed = append(result.Completed, SourceMutationItem{
				SourceID: lookupExtendSourceID(manifest, done),
				Group:    done.Group.String(),
				Status:   string(done.Status),
			})
		}
		result.Message = fmt.Sprintf("%d source(s) extended to %s: %d created, %d already installed",
			len(result.Completed), tool, result.CreatedLinks, result.AlreadyInstalled)
		if disabled > 0 {
			result.Message += fmt.Sprintf(", %d disabled", disabled)
		}
		result.Message += "."
		if applyErr != nil {
			var failure *install.ExtendFailure
			group := ""
			if errors.As(applyErr, &failure) {
				group = failure.Group.String()
			}
			result.Failure = &SourceMutationFailure{Stage: "extend", Group: group, Message: applyErr.Error(), RolledBack: applied.RolledBack}
			return applyErr
		}
		return nil
	})
	if err != nil {
		if result.Failure == nil {
			result.Failure = &SourceMutationFailure{Stage: "preflight", Message: err.Error()}
		}
		if result.Message == "" {
			result.Message = "Extend failed."
		}
	} else {
		s.emitProgress(SourceProgress{Operation: "extend", Phase: "done", Message: result.Message})
	}
	if !refreshed {
		s.attachCurrentSnapshot(&result)
	}
	return result
}

func projectExtendPreview(plan install.ExtendPlan) ExtendPreview {
	preview := ExtendPreview{Tool: plan.Tool.String(), Sources: []ExtendPreviewSource{}}
	for _, source := range plan.Sources {
		projected := ExtendPreviewSource{
			Kind:       string(source.Kind),
			Group:      source.Group.String(),
			SkillNames: []string{},
			Status:     string(source.Status),
			Reason:     source.Reason,
			Skipped:    []ExtendSkip{},
		}
		for _, link := range source.Links {
			projected.SkillNames = append(projected.SkillNames, link.Skill.Name)
		}
		for _, already := range source.AlreadyInstalled {
			projected.SkillNames = append(projected.SkillNames, already.Skill.Name)
		}
		projected.SkillCount = len(projected.SkillNames)
		projected.Created = len(source.Links)
		projected.AlreadyInstalled = len(source.AlreadyInstalled)
		projected.DisabledAfter = len(source.DisableAfter)
		for _, skipped := range source.Skipped {
			projected.Skipped = append(projected.Skipped, ExtendSkip{SkillName: skipped.SkillName, Reason: skipped.Reason})
		}
		projected.Conflicts = extendPlanConflicts(source.Err)
		preview.Sources = append(preview.Sources, projected)
		preview.CreateCount += projected.Created
		if source.Status == install.ExtendStatusBlocked {
			preview.BlockedCount++
		}
	}
	return preview
}

func extendPlanConflicts(err error) []InstallConflict {
	if err == nil {
		return nil
	}
	var planErr install.PlanError
	if !errors.As(err, &planErr) {
		return nil
	}
	conflicts := make([]InstallConflict, 0, len(planErr.Conflicts)+len(planErr.MissingSkills))
	for _, conflict := range planErr.Conflicts {
		conflicts = append(conflicts, InstallConflict{SkillName: conflict.SkillName, Tool: conflict.Tool.String(), Reason: conflict.Reason, Path: conflict.TargetPath})
	}
	for _, name := range planErr.MissingSkills {
		conflicts = append(conflicts, InstallConflict{SkillName: name, Reason: "skill is no longer present in the source"})
	}
	return conflicts
}

func lookupExtendSourceID(manifest state.Manifest, done install.ExtendSourceResult) string {
	if done.Kind == install.ExtendSourceGit {
		for _, repository := range manifest.Repositories {
			location := repository.OriginalURL
			if location == "" {
				location = repository.CanonicalURL
			}
			if location == done.Location {
				return repositorySourceID(repository)
			}
		}
	}
	if done.Kind == install.ExtendSourceLocal {
		for _, source := range manifest.LocalSources {
			if source.CanonicalPath == done.Location {
				return localSourceID(source)
			}
		}
	}
	return ""
}
