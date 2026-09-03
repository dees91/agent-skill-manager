package main

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/dees91/agent-skill-manager/internal/gui"
)

// App is the narrow Wails binding layer. All domain and filesystem behavior
// remains in the independently tested gui.Service.
type App struct {
	service *gui.Service
	mu      sync.RWMutex
	ctx     context.Context
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
	a.service.SetSourceProgressHandler(func(progress gui.SourceProgress) {
		runtime.EventsEmit(ctx, "source-operation-progress", progress)
	})
}

func newApp(service *gui.Service) *App {
	return &App{service: service}
}

// GetSnapshot refreshes the filesystem projection shown by the desktop app.
func (a *App) GetSnapshot(includeReadOnly bool) (gui.Snapshot, error) {
	return a.service.GetSnapshot(includeReadOnly)
}

// MeasureContextBudgets runs the explicitly requested local provider diagnostics.
func (a *App) MeasureContextBudgets() (gui.Snapshot, error) {
	return a.service.MeasureContextBudgets()
}

// ToggleCell stages or unstages the natural operation for one tool cell.
func (a *App) ToggleCell(skillName, tool string) (gui.ActionResult, error) {
	return a.service.ToggleCell(skillName, tool)
}

// ToggleBoth stages or unstages operations for both existing cells in a row.
func (a *App) ToggleBoth(skillName string) (gui.ActionResult, error) {
	return a.service.ToggleBoth(skillName)
}

// ToggleGroup smart-toggles a complete loaded group across both tools.
func (a *App) ToggleGroup(group string) (gui.ActionResult, error) {
	return a.service.ToggleGroup(group)
}

// ToggleGroupScope smart-toggles a complete group for selected tools.
func (a *App) ToggleGroupScope(group string, tools []string) (gui.ActionResult, error) {
	return a.service.ToggleGroupScope(group, tools)
}

// ToggleVisible smart-toggles the exact filtered row scope supplied by the UI.
func (a *App) ToggleVisible(skillNames []string) (gui.ActionResult, error) {
	return a.service.ToggleVisible(skillNames)
}

// ToggleSkillScope smart-toggles exact skill names for selected tools.
func (a *App) ToggleSkillScope(skillNames, tools []string) (gui.ActionResult, error) {
	return a.service.ToggleSkillScope(skillNames, tools)
}

// UndoCell removes one pending operation.
func (a *App) UndoCell(skillName, tool string) (gui.ActionResult, error) {
	return a.service.UndoCell(skillName, tool)
}

// ClearPending removes every pending operation in this desktop session.
func (a *App) ClearPending() gui.ActionResult {
	return a.service.ClearPending()
}

// ApplyPending applies the deterministic pending batch and returns a fresh scan.
func (a *App) ApplyPending(includeReadOnly bool) gui.ApplyResult {
	return a.service.ApplyPending(includeReadOnly)
}

// CreateSkillSet persists one task recipe from validated skill basenames.
func (a *App) CreateSkillSet(name, description string, skillNames []string) (gui.SkillSetMutationResult, error) {
	return a.service.CreateSkillSet(name, description, skillNames)
}

// UpdateSkillSet edits one recipe resolved by its opaque identifier.
func (a *App) UpdateSkillSet(setID, name, description string, skillNames []string) (gui.SkillSetMutationResult, error) {
	return a.service.UpdateSkillSet(setID, name, description, skillNames)
}

// DeleteSkillSet removes recipe metadata without changing skills or Pending.
func (a *App) DeleteSkillSet(setID string) (gui.SkillSetMutationResult, error) {
	return a.service.DeleteSkillSet(setID)
}

// PreviewSkillSetToggle returns a path-free staging preview for selected tools.
func (a *App) PreviewSkillSetToggle(setID string, tools []string) (gui.SkillSetTogglePreview, error) {
	return a.service.PreviewSkillSetToggle(setID, tools)
}

// ToggleSkillSet stages one scoped smart-toggle through the shared engine.
func (a *App) ToggleSkillSet(setID string, tools []string) (gui.ActionResult, error) {
	return a.service.ToggleSkillSet(setID, tools)
}

// SetSkillFavorite updates one tool-agnostic bookmark by skill basename.
func (a *App) SetSkillFavorite(skillName string, favorite bool) (gui.FavoriteMutationResult, error) {
	return a.service.SetSkillFavorite(skillName, favorite)
}

// PrepareGitInstall validates, clones/reuses, and discovers a Git source.
func (a *App) PrepareGitInstall(gitURL string) (gui.InstallDraft, error) {
	return a.service.PrepareGitInstall(gitURL)
}

// ChooseLocalInstall uses the native macOS picker before backend validation.
func (a *App) ChooseLocalInstall() (gui.InstallDraft, error) {
	a.mu.RLock()
	ctx := a.ctx
	a.mu.RUnlock()
	if ctx == nil {
		return gui.InstallDraft{}, context.Canceled
	}
	selected, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: "Choose a skill source folder"})
	if err != nil {
		return gui.InstallDraft{}, err
	}
	return a.service.PrepareLocalInstall(selected)
}

// ReviewInstall preflights an exact skill/tool matrix selection.
func (a *App) ReviewInstall(draftID string, selections []gui.InstallCellRequest) (gui.InstallReview, error) {
	return a.service.ReviewInstall(draftID, selections)
}

// ApplyInstall applies one immutable reviewed selection.
func (a *App) ApplyInstall(reviewID string, includeReadOnly bool) gui.SourceMutationResult {
	return a.service.ApplyInstall(reviewID, includeReadOnly)
}

// UpdateSource fetches and fast-forwards one managed Git source.
func (a *App) UpdateSource(sourceID string, includeReadOnly bool) gui.SourceMutationResult {
	return a.service.UpdateSource(sourceID, includeReadOnly)
}

// UpdateAllSources updates managed Git sources in manifest order.
func (a *App) UpdateAllSources(includeReadOnly bool) gui.SourceMutationResult {
	return a.service.UpdateAllSources(includeReadOnly)
}

// PreviewExtend reports the read-only impact of linking every managed source to one tool.
func (a *App) PreviewExtend(tool string) (gui.ExtendPreview, error) {
	return a.service.PreviewExtend(tool)
}

// ExtendSources links every managed source to one tool, stopping at the first failure.
func (a *App) ExtendSources(tool string) (gui.SourceMutationResult, error) {
	return a.service.ExtendSources(tool)
}

// PreviewUninstall audits exact owned links and reports uninstall impact.
func (a *App) PreviewUninstall(sourceID string) (gui.UninstallPreview, error) {
	return a.service.PreviewUninstall(sourceID)
}

// UninstallSource removes a whole managed source after typed confirmation.
func (a *App) UninstallSource(sourceID, confirmation string, includeReadOnly bool) gui.SourceMutationResult {
	return a.service.UninstallSource(sourceID, confirmation, includeReadOnly)
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.service.SourceBusy() {
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "Source operation in progress",
			Message: "Wait for the current install, update, or uninstall operation to finish before closing Skill Manager.",
			Buttons: []string{"OK"},
		})
		return true
	}
	if a.service.PendingCount() == 0 {
		return false
	}
	selection, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Discard pending changes?",
		Message:       "Skill Manager has unapplied changes. Closing now will discard them.",
		Buttons:       []string{"Keep Editing", "Discard Changes"},
		DefaultButton: "Keep Editing",
		CancelButton:  "Keep Editing",
	})
	if err != nil {
		return true
	}
	return selection != "Discard Changes"
}
