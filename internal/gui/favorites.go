package gui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/favorites"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// SetSkillFavorite idempotently updates one tool-agnostic skill bookmark.
func (s *Service) SetSkillFavorite(skillName string, favorite bool) (FavoriteMutationResult, error) {
	if s.SourceBusy() {
		return FavoriteMutationResult{}, fmt.Errorf("a source operation is in progress")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SourceBusy() {
		return FavoriteMutationResult{}, fmt.Errorf("a source operation is in progress")
	}
	if s.scannedAt.IsZero() {
		if err := s.reloadLocked(false); err != nil {
			return FavoriteMutationResult{}, err
		}
	}
	if s.favoritesWarning != "" {
		return FavoriteMutationResult{}, fmt.Errorf("favorites are unavailable: %s", s.favoritesWarning)
	}
	skillName = strings.TrimSpace(skillName)
	if favorite {
		row, err := s.rowLocked(skillName)
		if err != nil {
			return FavoriteMutationResult{}, err
		}
		if !favoriteEligible(row) {
			return FavoriteMutationResult{}, fmt.Errorf("skill %q is not a managed user skill", skillName)
		}
	}
	file, err := s.favoriteStore.Set(skillName, favorite)
	if err != nil {
		_ = s.refreshFavoritesLocked()
		return FavoriteMutationResult{}, err
	}
	s.favoriteFile = file
	s.favoritesWarning = ""
	action := "Removed"
	preposition := "from"
	if favorite {
		action = "Added"
		preposition = "to"
	}
	return s.favoriteMutationResultLocked(fmt.Sprintf("%s %s %s favorites.", action, skillName, preposition)), nil
}

func (s *Service) refreshFavoritesLocked() error {
	file, err := s.favoriteStore.Load()
	if err != nil {
		s.favoriteFile = favorites.File{}
		s.favoritesWarning = err.Error()
		return err
	}
	s.favoriteFile = file
	s.favoritesWarning = ""
	return nil
}

func (s *Service) reloadFavoritesLocked() {
	_ = s.refreshFavoritesLocked()
}

func (s *Service) favoriteMutationResultLocked(message string) FavoriteMutationResult {
	favorites := append([]string{}, s.favoriteFile.Skills...)
	return FavoriteMutationResult{Message: message, Favorites: favorites, Warning: s.favoritesWarning}
}

func favoriteEligible(row model.SkillRow) bool {
	for _, cell := range []*model.ToolSkill{row.Claude, row.Codex} {
		if cell == nil || cell.ReadOnly || cell.State == model.SkillStateReadOnly {
			continue
		}
		if cell.State == model.SkillStateOn || cell.State == model.SkillStateOff || cell.State == model.SkillStateConflict || cell.Conflict != nil {
			return true
		}
	}
	return false
}

func (s *Service) sourceFavoriteImpacts(installed []state.InstalledSkillEntry) ([]string, string) {
	file, err := s.favoriteStore.Load()
	if err != nil {
		return []string{}, err.Error()
	}
	installedNames := make(map[string]bool, len(installed))
	for _, skill := range installed {
		installedNames[skill.Name] = true
	}
	impacts := make([]string, 0)
	for _, name := range file.Skills {
		if installedNames[name] {
			impacts = append(impacts, name)
		}
	}
	sort.Strings(impacts)
	return impacts, ""
}
