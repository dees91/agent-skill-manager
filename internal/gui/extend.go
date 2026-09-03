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
		result = projectExtendPreview(manifest, plan)
		return nil
	})
	return result, err
}

// ExtendSources links every recorded skill to one tool in manifest order and
// stops at the first failed source, keeping the completed prefix.
func (s *Service) ExtendSources(toolName string) (SourceMutationResult, error) {
	result := SourceMutationResult{Completed: []SourceMutationItem{}}
	tool, ok := model.ParseTool(toolName)
	if !ok {
		return result, fmt.Errorf("unknown tool %q (supported: %s)", toolName, supportedExtendTools())
	}
	err := s.runSourceOperation("extend", "", func() error {
		defer s.refreshSourceResult(&result, false)
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
		result.Message = fmt.Sprintf("%d source(s) extended to %s: %d created, %d already installed.",
			len(result.Completed), tool, result.CreatedLinks, result.AlreadyInstalled)
		if disabled > 0 {
			result.Message += fmt.Sprintf("; %d disabled.", disabled)
		}
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
		return result, err
	}
	s.emitProgress(SourceProgress{Operation: "extend", Phase: "done", Message: result.Message})
	return result, nil
}

func projectExtendPreview(manifest state.Manifest, plan install.ExtendPlan) ExtendPreview {
	preview := ExtendPreview{Tool: plan.Tool.String(), Sources: []ExtendPreviewSource{}}
	repoIndex, localIndex := 0, 0
	for _, source := range plan.Sources {
		projected := ExtendPreviewSource{
			Kind:       string(source.Kind),
			Group:      source.Group.String(),
			SkillNames: []string{},
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
		preview.Sources = append(preview.Sources, projected)
		toolsRecorded := map[model.Tool]bool{}
		switch source.Kind {
		case install.ExtendSourceGit:
			if repoIndex < len(manifest.Repositories) {
				for _, skill := range manifest.Repositories[repoIndex].InstalledSkills {
					for _, tool := range skill.Tools {
						toolsRecorded[tool] = true
					}
				}
			}
			repoIndex++
		case install.ExtendSourceLocal:
			if localIndex < len(manifest.LocalSources) {
				for _, skill := range manifest.LocalSources[localIndex].InstalledSkills {
					for _, tool := range skill.Tools {
						toolsRecorded[tool] = true
					}
				}
			}
			localIndex++
		}
		if len(toolsRecorded) < len(model.Tools()) {
			preview.MuseCount++
		}
	}
	return preview
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
