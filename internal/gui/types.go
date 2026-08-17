package gui

import (
	"sort"
	"time"

	"github.com/dees91/agent-skill-manager/internal/contextbudget"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/staging"
)

// DiscoverToolState is the local install state for one catalog skill target.
type DiscoverToolState struct {
	Tool    string `json:"tool"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// DiscoverSkill is one path-free skills.sh catalog entry enriched with local state.
type DiscoverSkill struct {
	ID                string            `json:"id"`
	SkillID           string            `json:"skillId"`
	Name              string            `json:"name"`
	Source            string            `json:"source"`
	Installs          int64             `json:"installs"`
	WeeklyInstalls    []int64           `json:"weeklyInstalls,omitempty"`
	InstallsYesterday int64             `json:"installsYesterday,omitempty"`
	Change            int64             `json:"change,omitempty"`
	SourceType        string            `json:"sourceType"`
	URL               string            `json:"url"`
	Installable       bool              `json:"installable"`
	Claude            DiscoverToolState `json:"claude"`
	Codex             DiscoverToolState `json:"codex"`
}

// DiscoverPage is one leaderboard or search result with connection evidence.
type DiscoverPage struct {
	View       string          `json:"view"`
	Page       int             `json:"page"`
	Total      int             `json:"total"`
	HasMore    bool            `json:"hasMore"`
	Skills     []DiscoverSkill `json:"skills"`
	FetchedAt  time.Time       `json:"fetchedAt"`
	Offline    bool            `json:"offline"`
	FromCache  bool            `json:"fromCache"`
	SearchType string          `json:"searchType,omitempty"`
	Warning    string          `json:"warning,omitempty"`
}

// DiscoverDetail is the catalog detail drawer projection.
type DiscoverDetail struct {
	Skill       DiscoverSkill `json:"skill"`
	Description string        `json:"description,omitempty"`
	FetchedAt   time.Time     `json:"fetchedAt"`
	Offline     bool          `json:"offline"`
	FromCache   bool          `json:"fromCache"`
	AuditStatus string        `json:"auditStatus"`
	Warning     string        `json:"warning,omitempty"`
}

// Snapshot is the complete serializable projection consumed by the frontend.
type Snapshot struct {
	Rows             []SkillRow            `json:"rows"`
	SkillSets        []SkillSet            `json:"skillSets"`
	SkillSetsWarning string                `json:"skillSetsWarning,omitempty"`
	Groups           []GroupSummary        `json:"groups"`
	Sources          []string              `json:"sources"`
	ManagedSources   []ManagedSource       `json:"managedSources"`
	Stats            DashboardStats        `json:"stats"`
	Conflicts        []ConflictSummary     `json:"conflicts"`
	ContextBudgets   contextbudget.Reports `json:"contextBudgets"`
	Pending          []PendingChange       `json:"pending"`
	IncludeReadOnly  bool                  `json:"includeReadOnly"`
	ScannedAt        string                `json:"scannedAt"`
}

// SkillSet is one reusable task recipe enriched with current tool state.
type SkillSet struct {
	SetID       string              `json:"setId"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Members     []SkillSetMember    `json:"members"`
	Claude      SkillSetToolSummary `json:"claude"`
	Codex       SkillSetToolSummary `json:"codex"`
	Unavailable int                 `json:"unavailable"`
	Pending     int                 `json:"pending"`
	CreatedAt   string              `json:"createdAt"`
	UpdatedAt   string              `json:"updatedAt"`
}

// SkillSetMember projects one saved basename without exposing filesystem paths.
type SkillSetMember struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Group       string             `json:"group"`
	Source      string             `json:"source"`
	Available   bool               `json:"available"`
	Claude      SkillSetMemberCell `json:"claude"`
	Codex       SkillSetMemberCell `json:"codex"`
}

// SkillSetMemberCell is the status of one saved name for one tool.
type SkillSetMemberCell struct {
	Tool           string `json:"tool"`
	State          string `json:"state"`
	EffectiveState string `json:"effectiveState"`
	Pending        string `json:"pending,omitempty"`
	Eligible       bool   `json:"eligible"`
	Reason         string `json:"reason,omitempty"`
}

// SkillSetToolSummary describes applied and projected state for one tool.
type SkillSetToolSummary struct {
	Tool            string `json:"tool"`
	AppliedStatus   string `json:"appliedStatus"`
	EffectiveStatus string `json:"effectiveStatus"`
	Eligible        int    `json:"eligible"`
	On              int    `json:"on"`
	Off             int    `json:"off"`
	EffectiveOn     int    `json:"effectiveOn"`
	EffectiveOff    int    `json:"effectiveOff"`
	Pending         int    `json:"pending"`
	Missing         int    `json:"missing"`
	ReadOnly        int    `json:"readOnly"`
	Conflict        int    `json:"conflict"`
}

// SkillSetMutationResult returns refreshed recipes after metadata changes.
type SkillSetMutationResult struct {
	Message   string     `json:"message"`
	SkillSets []SkillSet `json:"skillSets"`
	Warning   string     `json:"warning,omitempty"`
}

// SkillSetTogglePreview explains one reversible staging action before it runs.
type SkillSetTogglePreview struct {
	SetID     string       `json:"setId"`
	Name      string       `json:"name"`
	Tools     []string     `json:"tools"`
	Direction string       `json:"direction"`
	Eligible  int          `json:"eligible"`
	Counts    ActionCounts `json:"counts"`
}

// ManagedSource is one Skill Manager-owned Git or local source. SourceID is
// the only identifier accepted back from the frontend for lifecycle actions.
type ManagedSource struct {
	SourceID    string `json:"sourceId"`
	Kind        string `json:"kind"`
	Group       string `json:"group"`
	Location    string `json:"location"`
	SkillCount  int    `json:"skillCount"`
	ClaudeCount int    `json:"claudeCount"`
	CodexCount  int    `json:"codexCount"`
	InstalledAt string `json:"installedAt"`
	Commit      string `json:"commit,omitempty"`
	CanUpdate   bool   `json:"canUpdate"`
	UpdateMode  string `json:"updateMode"`
	UpdateHint  string `json:"updateHint"`
}

// InstallCellRequest is one exact skill/tool selection from an inspected draft.
type InstallCellRequest struct {
	SkillName string `json:"skillName"`
	Tool      string `json:"tool"`
}

// InstallCandidateCell describes preflight state for one tool target.
type InstallCandidateCell struct {
	Tool    string `json:"tool"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// InstallCandidate is one discovered skill exposed without its absolute path.
type InstallCandidate struct {
	Name         string               `json:"name"`
	RelativePath string               `json:"relativePath"`
	Claude       InstallCandidateCell `json:"claude"`
	Codex        InstallCandidateCell `json:"codex"`
}

// InstallDraft is the inspected source and its selectable matrix.
type InstallDraft struct {
	DraftID       string             `json:"draftId"`
	Kind          string             `json:"kind"`
	Group         string             `json:"group"`
	Location      string             `json:"location"`
	Candidates    []InstallCandidate `json:"candidates"`
	Cloned        bool               `json:"cloned"`
	Reused        bool               `json:"reused"`
	RetainedClone bool               `json:"retainedClone"`
	Cancelled     bool               `json:"cancelled"`
}

// InstallConflict is one selected cell that failed review preflight.
type InstallConflict struct {
	SkillName string `json:"skillName"`
	Tool      string `json:"tool"`
	Reason    string `json:"reason"`
	Path      string `json:"path,omitempty"`
}

// InstallReview is an immutable, session-scoped reviewed selection.
type InstallReview struct {
	ReviewID        string               `json:"reviewId,omitempty"`
	DraftID         string               `json:"draftId"`
	Group           string               `json:"group"`
	Selections      []InstallCellRequest `json:"selections"`
	CreateCount     int                  `json:"createCount"`
	AlreadyOnCount  int                  `json:"alreadyOnCount"`
	AlreadyOffCount int                  `json:"alreadyOffCount"`
	Conflicts       []InstallConflict    `json:"conflicts"`
	Ready           bool                 `json:"ready"`
}

// SourceProgress is emitted during one non-cancellable lifecycle operation.
type SourceProgress struct {
	Operation string `json:"operation"`
	Phase     string `json:"phase"`
	Group     string `json:"group,omitempty"`
	Current   int    `json:"current,omitempty"`
	Total     int    `json:"total,omitempty"`
	Message   string `json:"message"`
}

// SourceMutationItem describes one source completed by an operation.
type SourceMutationItem struct {
	SourceID string `json:"sourceId"`
	Group    string `json:"group"`
	Status   string `json:"status"`
	Before   string `json:"before,omitempty"`
	After    string `json:"after,omitempty"`
}

// SourceMutationFailure preserves structured failure and cleanup information.
type SourceMutationFailure struct {
	Stage          string `json:"stage"`
	Group          string `json:"group,omitempty"`
	Message        string `json:"message"`
	RolledBack     int    `json:"rolledBack,omitempty"`
	CleanupPending string `json:"cleanupPending,omitempty"`
}

// SourceMutationResult returns the completed prefix and a fresh projection.
type SourceMutationResult struct {
	Message          string                 `json:"message"`
	Completed        []SourceMutationItem   `json:"completed"`
	Failure          *SourceMutationFailure `json:"failure,omitempty"`
	CreatedLinks     int                    `json:"createdLinks,omitempty"`
	AlreadyInstalled int                    `json:"alreadyInstalled,omitempty"`
	RemovedActive    int                    `json:"removedActive,omitempty"`
	RemovedDisabled  int                    `json:"removedDisabled,omitempty"`
	Snapshot         Snapshot               `json:"snapshot"`
}

// UninstallPreview is the validated impact displayed before typed confirmation.
type UninstallPreview struct {
	SourceID              string           `json:"sourceId"`
	Kind                  string           `json:"kind"`
	Group                 string           `json:"group"`
	Location              string           `json:"location"`
	ActiveLinks           int              `json:"activeLinks"`
	DisabledLinks         int              `json:"disabledLinks"`
	RemovesCheckout       bool             `json:"removesCheckout"`
	PreservesSource       bool             `json:"preservesSource"`
	AffectedSkillSets     []SkillSetImpact `json:"affectedSkillSets"`
	SkillSetImpactWarning string           `json:"skillSetImpactWarning,omitempty"`
}

// SkillSetImpact is a non-blocking source-uninstall dependency warning.
type SkillSetImpact struct {
	SetID  string   `json:"setId"`
	Name   string   `json:"name"`
	Skills []string `json:"skills"`
}

// SkillRow is one cross-tool row in the Skills table.
type SkillRow struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Source      string     `json:"source"`
	Group       string     `json:"group"`
	Claude      *SkillCell `json:"claude,omitempty"`
	Codex       *SkillCell `json:"codex,omitempty"`
}

// SkillCell contains the complete details for one tool-specific entry.
type SkillCell struct {
	Tool           string    `json:"tool"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Description    string    `json:"description"`
	State          string    `json:"state"`
	EffectiveState string    `json:"effectiveState"`
	Pending        string    `json:"pending,omitempty"`
	Source         string    `json:"source"`
	Group          string    `json:"group"`
	EntryType      string    `json:"entryType"`
	ActivePath     string    `json:"activePath"`
	DisabledPath   string    `json:"disabledPath"`
	SkillFilePath  string    `json:"skillFilePath"`
	SymlinkTarget  string    `json:"symlinkTarget"`
	RepoOrigin     string    `json:"repoOrigin"`
	RepoCommit     string    `json:"repoCommit"`
	ReadOnly       bool      `json:"readOnly"`
	Conflict       *Conflict `json:"conflict,omitempty"`
}

// Conflict describes a blocked restore cell.
type Conflict struct {
	OriginalPath string `json:"originalPath"`
	DisabledPath string `json:"disabledPath"`
	BlockerPath  string `json:"blockerPath"`
	Message      string `json:"message"`
}

// StateCounts summarizes one tool column.
type StateCounts struct {
	On       int `json:"on"`
	Off      int `json:"off"`
	Conflict int `json:"conflict"`
	ReadOnly int `json:"readOnly"`
}

// DashboardStats contains the top-level operational metrics.
type DashboardStats struct {
	ManagedSkills  int         `json:"managedSkills"`
	ReadOnlySkills int         `json:"readOnlySkills"`
	Claude         StateCounts `json:"claude"`
	Codex          StateCounts `json:"codex"`
	ConflictCells  int         `json:"conflictCells"`
}

// GroupSummary is a group panel row sorted by size, then name.
type GroupSummary struct {
	Group   string      `json:"group"`
	Rows    int         `json:"rows"`
	Claude  StateCounts `json:"claude"`
	Codex   StateCounts `json:"codex"`
	Sources []string    `json:"sources"`
}

// ConflictSummary is the compact dashboard representation of a conflict.
type ConflictSummary struct {
	Tool        string `json:"tool"`
	SkillName   string `json:"skillName"`
	Group       string `json:"group"`
	BlockerPath string `json:"blockerPath"`
	Message     string `json:"message"`
}

// PendingChange is a path-free pending operation exposed to the frontend.
type PendingChange struct {
	Tool      string `json:"tool"`
	SkillName string `json:"skillName"`
	Operation string `json:"operation"`
}

// ActionCounts reports staging updates and skipped cells.
type ActionCounts struct {
	Changed         int `json:"changed"`
	Removed         int `json:"removed"`
	SkippedReadOnly int `json:"skippedReadOnly"`
	SkippedMissing  int `json:"skippedMissing"`
	SkippedConflict int `json:"skippedConflict"`
}

// ActionResult is returned by staging and undo operations.
type ActionResult struct {
	Message          string                `json:"message"`
	Counts           ActionCounts          `json:"counts"`
	Pending          []PendingChange       `json:"pending"`
	ContextBudgets   contextbudget.Reports `json:"contextBudgets"`
	SkillSets        []SkillSet            `json:"skillSets"`
	SkillSetsWarning string                `json:"skillSetsWarning,omitempty"`
}

// AppliedChange records one completed filesystem operation.
type AppliedChange struct {
	Tool      string `json:"tool"`
	SkillName string `json:"skillName"`
	Operation string `json:"operation"`
}

// ApplyFailure describes an expected preflight/apply/rescan failure.
type ApplyFailure struct {
	Stage     string `json:"stage"`
	Tool      string `json:"tool,omitempty"`
	SkillName string `json:"skillName,omitempty"`
	Operation string `json:"operation,omitempty"`
	Message   string `json:"message"`
}

// ApplyResult preserves the completed prefix and a fresh snapshot.
type ApplyResult struct {
	Completed []AppliedChange `json:"completed"`
	Failure   *ApplyFailure   `json:"failure,omitempty"`
	Message   string          `json:"message"`
	Snapshot  Snapshot        `json:"snapshot"`
}

func projectRow(row model.SkillRow, pending staging.Memory) SkillRow {
	return SkillRow{
		Name:        row.Name,
		Description: row.Description,
		Source:      row.Source.String(),
		Group:       normalizedGroup(row.Group).String(),
		Claude:      projectCell(row.Claude, pending),
		Codex:       projectCell(row.Codex, pending),
	}
}

func projectCell(cell *model.ToolSkill, pending staging.Memory) *SkillCell {
	if cell == nil {
		return nil
	}
	kind := pending[staging.Key{Tool: cell.Tool, SkillName: cell.Name}]
	projected := &SkillCell{
		Tool:           cell.Tool.String(),
		Name:           cell.Name,
		DisplayName:    cell.DisplayName,
		Description:    cell.Description,
		State:          cell.State.String(),
		EffectiveState: staging.EffectiveState(cell, kind).String(),
		Pending:        kind.String(),
		Source:         cell.Source.String(),
		Group:          normalizedGroup(cell.Group).String(),
		EntryType:      cell.EntryType.String(),
		ActivePath:     cell.ActivePath,
		DisabledPath:   cell.DisabledPath,
		SkillFilePath:  cell.SkillFilePath,
		SymlinkTarget:  cell.SymlinkTarget,
		RepoOrigin:     cell.RepoOrigin,
		RepoCommit:     cell.RepoCommit,
		ReadOnly:       cell.ReadOnly,
	}
	if cell.Conflict != nil {
		projected.Conflict = &Conflict{
			OriginalPath: cell.Conflict.OriginalPath,
			DisabledPath: cell.Conflict.DisabledPath,
			BlockerPath:  cell.Conflict.BlockerPath,
			Message:      cell.Conflict.Message,
		}
	}
	return projected
}

func projectGroups(groups []model.GroupSummary) []GroupSummary {
	result := make([]GroupSummary, 0, len(groups))
	for _, group := range groups {
		sources := make([]string, 0, len(group.Sources))
		for _, source := range group.Sources {
			sources = append(sources, source.String())
		}
		result = append(result, GroupSummary{
			Group:   normalizedGroup(group.Group).String(),
			Rows:    group.Rows,
			Claude:  projectCounts(group.Claude),
			Codex:   projectCounts(group.Codex),
			Sources: sources,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Rows != result[j].Rows {
			return result[i].Rows > result[j].Rows
		}
		return result[i].Group < result[j].Group
	})
	return result
}

func projectCounts(counts model.ToolStateCounts) StateCounts {
	return StateCounts{On: counts.On, Off: counts.Off, Conflict: counts.Conflict, ReadOnly: counts.ReadOnly}
}

func summarize(rows []model.SkillRow) (DashboardStats, []ConflictSummary) {
	stats := DashboardStats{}
	conflicts := []ConflictSummary{}
	for _, row := range rows {
		managed := false
		for _, cell := range []*model.ToolSkill{row.Claude, row.Codex} {
			if cell == nil {
				continue
			}
			counts := &stats.Claude
			if cell.Tool == model.ToolCodex {
				counts = &stats.Codex
			}
			switch cell.State {
			case model.SkillStateOn:
				counts.On++
				managed = true
			case model.SkillStateOff:
				counts.Off++
				managed = true
			case model.SkillStateConflict:
				counts.Conflict++
				stats.ConflictCells++
				managed = true
				conflict := ConflictSummary{Tool: cell.Tool.String(), SkillName: row.Name, Group: normalizedGroup(row.Group).String()}
				if cell.Conflict != nil {
					conflict.BlockerPath = cell.Conflict.BlockerPath
					conflict.Message = cell.Conflict.Message
				}
				conflicts = append(conflicts, conflict)
			case model.SkillStateReadOnly:
				counts.ReadOnly++
			}
		}
		if managed {
			stats.ManagedSkills++
		} else {
			stats.ReadOnlySkills++
		}
	}
	sort.SliceStable(conflicts, func(i, j int) bool {
		if conflicts[i].SkillName != conflicts[j].SkillName {
			return conflicts[i].SkillName < conflicts[j].SkillName
		}
		return conflicts[i].Tool < conflicts[j].Tool
	})
	return stats, conflicts
}

func collectSources(rows []model.SkillRow) []string {
	seen := make(map[string]struct{})
	for _, row := range rows {
		for _, cell := range []*model.ToolSkill{row.Claude, row.Codex} {
			if cell != nil && cell.Source != "" {
				seen[cell.Source.String()] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for source := range seen {
		result = append(result, source)
	}
	sort.Strings(result)
	return result
}
