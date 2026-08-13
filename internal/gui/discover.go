package gui

import (
	"context"
	"fmt"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/skillssh"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// GetDiscoverPage loads one skills.sh leaderboard and enriches it with local state.
func (s *Service) GetDiscoverPage(view string, page int, forceRefresh bool) (DiscoverPage, error) {
	loaded, err := s.catalog.GetPage(context.Background(), skillssh.View(strings.TrimSpace(view)), page, forceRefresh)
	if err != nil {
		return DiscoverPage{}, err
	}
	return s.projectDiscoverPage(loaded)
}

// SearchDiscover searches skills.sh or the local catalog cache when offline.
func (s *Service) SearchDiscover(query string) (DiscoverPage, error) {
	loaded, err := s.catalog.Search(context.Background(), query)
	if err != nil {
		return DiscoverPage{}, err
	}
	return s.projectDiscoverPage(loaded)
}

// GetDiscoverSkill returns safe display metadata for one previously listed ID.
func (s *Service) GetDiscoverSkill(skillID string, forceRefresh bool) (DiscoverDetail, error) {
	skill, err := s.catalogSkill(skillID)
	if err != nil {
		return DiscoverDetail{}, err
	}
	loaded, err := s.catalog.GetDetail(context.Background(), skill, forceRefresh)
	if err != nil {
		return DiscoverDetail{}, err
	}
	projected, err := s.projectDiscoverSkill(loaded.Skill)
	if err != nil {
		return DiscoverDetail{}, err
	}
	return DiscoverDetail{Skill: projected, Description: loaded.Description, FetchedAt: loaded.FetchedAt, Offline: loaded.Offline, FromCache: loaded.FromCache, AuditStatus: loaded.AuditStatus, Warning: loaded.Warning}, nil
}

// InstallDiscoverSkill installs exactly one previously listed GitHub skill.
func (s *Service) InstallDiscoverSkill(skillID string, toolNames []string, includeReadOnly bool) SourceMutationResult {
	result := SourceMutationResult{Completed: []SourceMutationItem{}}
	skill, err := s.catalogSkill(skillID)
	if err != nil {
		result.Failure = &SourceMutationFailure{Stage: "catalog", Message: err.Error()}
		result.Message = "Install failed."
		s.attachCurrentSnapshot(&result)
		return result
	}
	tools, err := normalizeDiscoverTools(toolNames)
	if err != nil {
		result.Failure = &SourceMutationFailure{Stage: "preflight", Message: err.Error()}
		result.Message = "Install failed."
		s.attachCurrentSnapshot(&result)
		return result
	}

	refreshed := false
	err = s.runSourceOperation("install", skill.Source, func() error {
		defer func() {
			s.refreshSourceResult(&result, includeReadOnly)
			refreshed = true
		}()
		s.emitProgress(SourceProgress{Operation: "install", Phase: "catalog", Group: skill.Source, Message: "Revalidating the skills.sh catalog entry…"})
		detail, detailErr := s.catalog.GetDetail(context.Background(), skill, true)
		if detailErr != nil {
			return fmt.Errorf("revalidate skills.sh catalog entry: %w", detailErr)
		}
		if detail.Offline {
			return fmt.Errorf("skills.sh is offline; installation requires a live catalog response")
		}
		if skill.SourceType != "github" || skill.InstallURL == "" {
			return fmt.Errorf("well-known skills are not installable in this version")
		}
		projected, projectErr := s.projectDiscoverSkill(skill)
		if projectErr != nil {
			return projectErr
		}
		for _, tool := range tools {
			toolState := projected.Claude
			if tool == model.ToolCodex {
				toolState = projected.Codex
			}
			if toolState.Status != "available" {
				return fmt.Errorf("%s is not available for %s: %s", skill.Name, tool.String(), toolState.Status)
			}
		}

		identity, normalizeErr := install.NormalizeGitURL(skill.InstallURL)
		if normalizeErr != nil {
			return normalizeErr
		}
		checkoutPath, pathErr := install.CheckoutPath(s.paths, identity)
		if pathErr != nil {
			return pathErr
		}
		s.emitProgress(SourceProgress{Operation: "install", Phase: "checkout", Group: identity.Group.String(), Message: "Cloning or validating the managed checkout…"})
		checkout, checkoutErr := install.NewCheckoutService(s.gitRunner).EnsureCheckout(identity, checkoutPath, install.CheckoutOptions{})
		if checkoutErr != nil {
			return checkoutErr
		}
		s.emitProgress(SourceProgress{Operation: "install", Phase: "discover", Group: identity.Group.String(), Message: "Finding the selected skill in the repository…"})
		discovered, discoverErr := install.DiscoverSkills(checkoutPath)
		if discoverErr != nil {
			return discoverErr
		}
		cells := make([]install.InstallCell, 0, len(tools))
		for _, tool := range tools {
			cells = append(cells, install.InstallCell{SkillName: skill.SkillID, Tool: tool})
		}
		manifest, loadErr := s.store.Load()
		if loadErr != nil {
			return loadErr
		}
		plan, planErr := install.PlanInstall(s.paths, manifest, identity, checkoutPath, discovered, install.PlanOptions{Cells: cells})
		if planErr != nil {
			return planErr
		}
		s.emitProgress(SourceProgress{Operation: "install", Phase: "apply", Group: identity.Group.String(), Message: "Creating selected skill links and saving ownership…"})
		applied, applyErr := install.NewApplyService(s.paths).Apply(plan, checkout.LastSeenCommit)
		result.CreatedLinks = len(applied.Created)
		result.AlreadyInstalled = len(applied.AlreadyInstalled)
		if applyErr != nil {
			result.Failure = &SourceMutationFailure{Stage: "apply", Group: identity.Group.String(), Message: applyErr.Error(), RolledBack: len(applied.RolledBack)}
			return applyErr
		}
		result.Completed = append(result.Completed, SourceMutationItem{SourceID: repositorySourceID(applied.Repository), Group: identity.Group.String(), Status: "installed"})
		result.Message = fmt.Sprintf("Installed %s for %d agent(s); %d link(s) already installed.", skill.Name, len(tools), result.AlreadyInstalled)
		return nil
	})
	if err != nil {
		if result.Failure == nil {
			result.Failure = &SourceMutationFailure{Stage: "preflight", Group: skill.Source, Message: err.Error()}
		}
		result.Message = "Install failed."
	}
	if !refreshed {
		s.attachCurrentSnapshot(&result)
	}
	return result
}

func (s *Service) projectDiscoverPage(page skillssh.Page) (DiscoverPage, error) {
	result := DiscoverPage{View: string(page.View), Page: page.Page, Total: page.Total, HasMore: page.HasMore, FetchedAt: page.FetchedAt, Offline: page.Offline, FromCache: page.FromCache, SearchType: page.SearchType, Warning: page.Warning, Skills: make([]DiscoverSkill, 0, len(page.Skills))}
	manifest, rows, err := s.discoverProjectionContext()
	if err != nil {
		return DiscoverPage{}, err
	}
	for _, skill := range page.Skills {
		result.Skills = append(result.Skills, projectDiscoverSkillWithState(skill, rows, manifest))
	}
	s.mu.Lock()
	for _, skill := range page.Skills {
		s.catalogSkills[skill.ID] = skill
	}
	s.mu.Unlock()
	return result, nil
}

func (s *Service) projectDiscoverSkill(skill skillssh.Skill) (DiscoverSkill, error) {
	manifest, rows, err := s.discoverProjectionContext()
	if err != nil {
		return DiscoverSkill{}, err
	}
	return projectDiscoverSkillWithState(skill, rows, manifest), nil
}

func (s *Service) discoverProjectionContext() (state.Manifest, []model.SkillRow, error) {
	manifest, err := s.store.Load()
	if err != nil {
		return state.Manifest{}, nil, err
	}
	s.mu.Lock()
	rows := append([]model.SkillRow(nil), s.rows...)
	s.mu.Unlock()
	return manifest, rows, nil
}

func projectDiscoverSkillWithState(skill skillssh.Skill, rows []model.SkillRow, manifest state.Manifest) DiscoverSkill {
	projected := DiscoverSkill{ID: skill.ID, SkillID: skill.SkillID, Name: skill.Name, Source: skill.Source, Installs: skill.Installs, WeeklyInstalls: append([]int64(nil), skill.WeeklyInstalls...), InstallsYesterday: skill.InstallsYesterday, Change: skill.Change, SourceType: skill.SourceType, URL: skill.URL, Installable: skill.SourceType == "github"}
	projected.Claude = discoverToolState(rows, manifest, skill, model.ToolClaude)
	projected.Codex = discoverToolState(rows, manifest, skill, model.ToolCodex)
	return projected
}

func discoverToolState(rows []model.SkillRow, manifest state.Manifest, skill skillssh.Skill, tool model.Tool) DiscoverToolState {
	result := DiscoverToolState{Tool: tool.String(), Status: "available"}
	var cell *model.ToolSkill
	for index := range rows {
		if rows[index].Name != skill.SkillID {
			continue
		}
		if tool == model.ToolClaude {
			cell = rows[index].Claude
		} else {
			cell = rows[index].Codex
		}
		break
	}
	repository, repositoryFound := manifestRepositoryForCatalog(manifest, skill.Source)
	owned := repositoryFound && repositoryOwnsCell(repository, skill.SkillID, tool)
	if cell == nil {
		if owned {
			result.Status = "conflict"
			result.Message = "Skill Manager records this target, but its filesystem entry is missing."
		}
		return result
	}
	if !owned {
		result.Status = "conflict"
		result.Message = "The same skill name already exists from another or unmanaged source."
		return result
	}
	if cell.State == model.SkillStateConflict || cell.Conflict != nil {
		result.Status = "conflict"
		result.Message = "The managed local entry is in conflict."
		return result
	}
	if cell.State == model.SkillStateOff {
		result.Status = "installed-off"
		return result
	}
	if cell.State == model.SkillStateOn {
		result.Status = "installed-on"
		return result
	}
	result.Status = "conflict"
	result.Message = "The local entry has an unsupported state."
	return result
}

func manifestRepositoryForCatalog(manifest state.Manifest, source string) (state.RepositoryEntry, bool) {
	identity, err := install.NormalizeGitURL("https://github.com/" + source)
	if err != nil {
		return state.RepositoryEntry{}, false
	}
	return manifest.GetRepository(identity.Host, identity.RepoPath)
}

func repositoryOwnsCell(repository state.RepositoryEntry, skillName string, tool model.Tool) bool {
	for _, installed := range repository.InstalledSkills {
		if installed.Name != skillName {
			continue
		}
		for _, installedTool := range installed.Tools {
			if installedTool == tool {
				return true
			}
		}
	}
	return false
}

func (s *Service) catalogSkill(id string) (skillssh.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	skill, ok := s.catalogSkills[strings.TrimSpace(id)]
	if !ok {
		return skillssh.Skill{}, fmt.Errorf("catalog skill is missing or expired; refresh Discover")
	}
	return skill, nil
}

func normalizeDiscoverTools(names []string) ([]model.Tool, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("select at least one agent")
	}
	seen := map[model.Tool]bool{}
	result := make([]model.Tool, 0, len(names))
	for _, name := range names {
		tool, ok := model.ParseTool(strings.ToLower(strings.TrimSpace(name)))
		if !ok {
			return nil, fmt.Errorf("unsupported agent %q", name)
		}
		if !seen[tool] {
			seen[tool] = true
			result = append(result, tool)
		}
	}
	return result, nil
}
