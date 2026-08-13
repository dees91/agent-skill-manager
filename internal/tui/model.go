package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	domain "github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/ops"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/scan"
	"github.com/dees91/agent-skill-manager/internal/staging"
)

type pendingKey struct {
	tool      domain.Tool
	skillName string
}

const allSourcesFilter domain.SourceLabel = ""
const allGroupsFilter domain.GroupLabel = ""

// Model is the Bubble Tea state for the main skills table.
type Model struct {
	paths         paths.Paths
	allRows       []domain.SkillRow
	rows          []domain.SkillRow
	cursor        int
	activeTool    domain.Tool
	width         int
	height        int
	pending       map[pendingKey]domain.OperationKind
	message       string
	confirmQuit   bool
	showReadOnly  bool
	showDetails   bool
	textFilter    string
	editingFilter bool
	sourceFilter  domain.SourceLabel
	sourceChoices []domain.SourceLabel
	groupFilter   domain.GroupLabel
	groupChoices  []domain.GroupLabel
	err           error
}

// New builds the initial TUI model from the filesystem scan.
func New(p paths.Paths) Model {
	m := Model{
		paths:      p,
		activeTool: domain.ToolClaude,
		pending:    make(map[pendingKey]domain.OperationKind),
	}
	m.reload()
	return m
}

// Run starts the interactive Bubble Tea program.
func Run(p paths.Paths) error {
	_, err := tea.NewProgram(New(p), tea.WithAltScreen()).Run()
	return err
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.editingFilter {
			m.updateFilterInput(msg)
			return m, nil
		}

		if key != "q" {
			m.confirmQuit = false
		}

		switch key {
		case "q":
			if len(m.pending) > 0 && !m.confirmQuit {
				m.confirmQuit = true
				m.message = "Pending changes; press q again to quit or U to clear."
				return m, nil
			}
			return m, tea.Quit
		case "tab":
			m.toggleActiveTool()
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case " ", "space":
			m.toggleActivePending()
		case "b":
			m.toggleBothPending()
		case "a", "enter":
			m.applyPending()
		case "u":
			m.undoActivePending()
		case "U":
			m.clearPending()
		case "o":
			m.toggleReadOnly()
		case "/":
			m.editingFilter = true
			m.message = "Filtering by text."
		case "s":
			m.cycleSourceFilter()
		case "g":
			m.toggleSelectedGroupPending()
		case "A":
			m.toggleAllVisiblePending()
		case "G":
			m.cycleGroupFilter()
		case "d":
			m.showDetails = !m.showDetails
		case "r":
			m.reload()
			m.message = "Rescanned from disk."
		}
	}
	return m, nil
}

func (m *Model) toggleActiveTool() {
	if m.activeTool == domain.ToolClaude {
		m.activeTool = domain.ToolCodex
		m.message = "Active column: Codex"
		return
	}
	m.activeTool = domain.ToolClaude
	m.message = "Active column: Claude"
}

func (m *Model) reload() {
	scanner := scan.New(m.paths)
	skills, err := scanner.Managed()
	if err == nil {
		var disabled []domain.ToolSkill
		disabled, err = scanner.Disabled()
		skills = append(skills, disabled...)
	}
	if err == nil && m.showReadOnly {
		var readOnly []domain.ToolSkill
		readOnly, err = scanner.ReadOnly()
		skills = append(skills, readOnly...)
	}
	if err != nil {
		m.err = err
		m.allRows = []domain.SkillRow{}
		m.rows = []domain.SkillRow{}
		m.sourceChoices = nil
		m.groupChoices = nil
		m.clampCursor()
		return
	}

	m.err = nil
	m.allRows = scan.RowsFromSkillsWithOptions(skills, scan.RowOptions{IncludeReadOnly: m.showReadOnly})
	m.rebuildRows()
}

func (m *Model) rebuildRows() {
	selectedName := ""
	if row, ok := m.selectedRow(); ok {
		selectedName = row.Name
	}

	textRows := make([]domain.SkillRow, 0, len(m.allRows))
	for _, row := range m.allRows {
		if rowMatchesTextFilter(row, m.textFilter) {
			textRows = append(textRows, row)
		}
	}

	m.sourceChoices = collectSourceChoices(textRows)
	if m.sourceFilter != allSourcesFilter && !sourceChoiceAvailable(m.sourceChoices, m.sourceFilter) {
		m.sourceFilter = allSourcesFilter
	}

	sourceRows := make([]domain.SkillRow, 0, len(textRows))
	for _, row := range textRows {
		if rowMatchesSourceFilter(row, m.sourceFilter) {
			sourceRows = append(sourceRows, row)
		}
	}

	m.groupChoices = collectGroupChoices(sourceRows)
	if m.groupFilter != allGroupsFilter && !groupChoiceAvailable(m.groupChoices, m.groupFilter) {
		m.groupFilter = allGroupsFilter
	}

	rows := make([]domain.SkillRow, 0, len(sourceRows))
	for _, row := range sourceRows {
		if rowMatchesGroupFilter(row, m.groupFilter) {
			rows = append(rows, row)
		}
	}
	m.rows = rows

	if selectedName != "" {
		for i, row := range m.rows {
			if row.Name == selectedName {
				m.cursor = i
				return
			}
		}
	}
	m.clampCursor()
}

func (m *Model) clampCursor() {
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

func (m *Model) toggleActivePending() {
	row, ok := m.selectedRow()
	if !ok {
		m.message = "No selected skill."
		return
	}

	result := m.togglePendingCell(row, m.activeTool)
	m.setToggleMessage(result, 1)
}

func (m *Model) toggleBothPending() {
	row, ok := m.selectedRow()
	if !ok {
		m.message = "No selected skill."
		return
	}

	changed := 0
	for _, tool := range domain.Tools() {
		if result := m.togglePendingCell(row, tool); result.changed {
			changed++
		}
	}
	if changed == 0 {
		m.message = "No toggleable cells in this row."
		return
	}
	m.setToggleMessage(toggleResult{changed: true}, changed)
}

func (m *Model) toggleSelectedGroupPending() {
	row, ok := m.selectedRow()
	if !ok {
		m.message = "No selected skill."
		return
	}

	group := normalizedGroup(row.Group)
	scopeRows := make([]domain.SkillRow, 0)
	for _, candidate := range m.allRows {
		if normalizedGroup(candidate.Group) == group {
			scopeRows = append(scopeRows, candidate)
		}
	}

	result := m.batchTogglePending(scopeRows, domain.Tools())
	m.message = formatBatchToggleMessage(fmt.Sprintf("Group %s", group), result)
}

func (m *Model) toggleAllVisiblePending() {
	if len(m.rows) == 0 {
		m.message = "No visible skills."
		return
	}

	result := m.batchTogglePending(m.rows, domain.Tools())
	m.message = formatBatchToggleMessage("All visible rows", result)
}

func formatBatchToggleMessage(scope string, result batchToggleResult) string {
	updated := result.changed + result.removed
	if updated == 0 {
		message := scope + ": no applicable cells."
		if skips := formatBatchSkipSummary(result); skips != "" {
			message += " Skipped " + skips + "."
		}
		return message
	}

	message := fmt.Sprintf("%s: %d %s updated", scope, updated, pluralize(updated, "pending change", "pending changes"))
	breakdown := formatBatchUpdateBreakdown(result)
	if breakdown != "" {
		message += " (" + breakdown + ")"
	}
	message += "."
	if skips := formatBatchSkipSummary(result); skips != "" {
		message += " Skipped " + skips + "."
	}
	return message
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func formatBatchUpdateBreakdown(result batchToggleResult) string {
	parts := []string{}
	if result.changed > 0 {
		parts = append(parts, fmt.Sprintf("%d changed", result.changed))
	}
	if result.removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", result.removed))
	}
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func formatBatchSkipSummary(result batchToggleResult) string {
	parts := []string{}
	if result.skippedConflict > 0 {
		parts = append(parts, fmt.Sprintf("%d conflict", result.skippedConflict))
	}
	if result.skippedReadOnly > 0 {
		parts = append(parts, fmt.Sprintf("%d read-only", result.skippedReadOnly))
	}
	if result.skippedMissing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", result.skippedMissing))
	}
	return strings.Join(parts, ", ")
}

type toggleResult struct {
	changed bool
	removed bool
}

type batchToggleResult struct {
	changed         int
	removed         int
	skippedReadOnly int
	skippedMissing  int
	skippedConflict int
}

type tuiPendingStore struct {
	model *Model
}

func (s tuiPendingStore) Get(key staging.Key) (domain.OperationKind, bool) {
	if s.model.pending == nil {
		return "", false
	}
	kind, ok := s.model.pending[pendingKey{tool: key.Tool, skillName: key.SkillName}]
	return kind, ok
}

func (s tuiPendingStore) Set(key staging.Key, kind domain.OperationKind) {
	if s.model.pending == nil {
		s.model.pending = make(map[pendingKey]domain.OperationKind)
	}
	s.model.pending[pendingKey{tool: key.Tool, skillName: key.SkillName}] = kind
}

func (s tuiPendingStore) Delete(key staging.Key) {
	delete(s.model.pending, pendingKey{tool: key.Tool, skillName: key.SkillName})
}

func (m *Model) togglePendingCell(row domain.SkillRow, tool domain.Tool) toggleResult {
	result := staging.ToggleCell(tuiPendingStore{model: m}, row, tool)
	m.clearConfirmQuitIfNoPending()
	return toggleResult{changed: result.Changed, removed: result.Removed}
}

func (m *Model) batchTogglePending(rows []domain.SkillRow, tools []domain.Tool) batchToggleResult {
	staged := staging.ToggleBatch(tuiPendingStore{model: m}, rows, tools)
	m.clearConfirmQuitIfNoPending()
	return batchToggleResult{
		changed:         staged.Changed,
		removed:         staged.Removed,
		skippedReadOnly: staged.SkippedReadOnly,
		skippedMissing:  staged.SkippedMissing,
		skippedConflict: staged.SkippedConflict,
	}
}

func (m *Model) setToggleMessage(result toggleResult, changed int) {
	switch {
	case !result.changed:
		m.message = "Active cell cannot be toggled."
	case result.removed && changed == 1:
		m.message = "Pending change removed."
	case changed == 1:
		m.message = "Pending change added."
	default:
		m.message = fmt.Sprintf("%d pending changes updated.", changed)
	}
}

func (m *Model) undoActivePending() {
	row, ok := m.selectedRow()
	if !ok {
		m.message = "No selected skill."
		return
	}

	key := pendingKey{tool: m.activeTool, skillName: row.Name}
	if _, ok := m.pending[key]; !ok {
		m.message = "No pending change for active cell."
		return
	}
	delete(m.pending, key)
	m.clearConfirmQuitIfNoPending()
	m.message = "Pending change removed."
}

func (m *Model) clearPending() {
	if len(m.pending) == 0 {
		m.message = "No pending changes to clear."
		return
	}
	m.pending = make(map[pendingKey]domain.OperationKind)
	m.confirmQuit = false
	m.message = "All pending changes cleared."
}

func (m *Model) applyPending() {
	if len(m.pending) == 0 {
		m.message = "No pending changes to apply."
		return
	}

	service := ops.New(m.paths)
	requests := make([]ops.PlanRequest, 0, len(m.pending))
	for _, key := range m.orderedPendingKeys() {
		kind := m.pending[key]
		requests = append(requests, ops.PlanRequest{
			Kind:      kind,
			Tool:      key.tool,
			SkillName: key.skillName,
		})
	}
	operations, err := service.PlanBatch(requests)
	if err != nil {
		m.message = fmt.Sprintf("Cannot apply: %v", err)
		return
	}

	result := service.Apply(operations)
	for _, completed := range result.Completed {
		delete(m.pending, pendingKey{tool: completed.Tool, skillName: completed.SkillName})
	}
	m.clearConfirmQuitIfNoPending()
	m.reload()

	if result.Failed != nil {
		failed := result.Failed.Operation
		m.message = fmt.Sprintf("Apply failed at %s %s/%s: %v", failed.Kind, failed.Tool, failed.SkillName, result.Failed.Err)
		return
	}

	m.message = fmt.Sprintf("Applied %d change(s).", len(result.Completed))
}

func (m Model) selectedRow() (domain.SkillRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return domain.SkillRow{}, false
	}
	return m.rows[m.cursor], true
}

func (m Model) orderedPendingKeys() []pendingKey {
	keys := make([]pendingKey, 0, len(m.pending))
	for key := range m.pending {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		leftKind := m.pending[keys[i]]
		rightKind := m.pending[keys[j]]
		if leftKind != rightKind {
			return pendingOperationRank(leftKind) < pendingOperationRank(rightKind)
		}
		if keys[i].tool != keys[j].tool {
			return keys[i].tool.String() < keys[j].tool.String()
		}
		return keys[i].skillName < keys[j].skillName
	})
	return keys
}

func pendingOperationRank(kind domain.OperationKind) int {
	switch kind {
	case domain.OperationDisable:
		return 0
	case domain.OperationEnable:
		return 1
	default:
		return 2
	}
}

func (m *Model) clearConfirmQuitIfNoPending() {
	if len(m.pending) == 0 {
		m.confirmQuit = false
	}
}

func (m *Model) toggleReadOnly() {
	m.showReadOnly = !m.showReadOnly
	m.reload()
	if m.showReadOnly {
		m.message = "Read-only entries shown."
		return
	}
	m.message = "Read-only entries hidden."
}

func (m *Model) updateFilterInput(msg tea.KeyMsg) {
	switch msg.String() {
	case "enter", "esc":
		m.editingFilter = false
		m.setTextFilterMessage()
	case "backspace", "ctrl+h":
		if m.textFilter != "" {
			runes := []rune(m.textFilter)
			m.textFilter = string(runes[:len(runes)-1])
			m.rebuildRows()
		}
		m.setTextFilterMessage()
	default:
		if msg.Type == tea.KeyRunes {
			m.textFilter += string(msg.Runes)
			m.rebuildRows()
			m.setTextFilterMessage()
		}
	}
}

func (m *Model) setTextFilterMessage() {
	if m.textFilter == "" {
		m.message = "Text filter cleared."
		return
	}
	m.message = fmt.Sprintf("Text filter: %s", m.textFilter)
}

func (m *Model) cycleSourceFilter() {
	if len(m.sourceChoices) == 0 {
		m.sourceFilter = allSourcesFilter
		m.message = "No source filters available."
		return
	}

	nextIndex := -1
	if m.sourceFilter == allSourcesFilter {
		nextIndex = 0
	} else {
		for i, source := range m.sourceChoices {
			if source == m.sourceFilter {
				nextIndex = i + 1
				break
			}
		}
	}

	if nextIndex < 0 || nextIndex >= len(m.sourceChoices) {
		m.sourceFilter = allSourcesFilter
		m.message = "Source filter: all"
	} else {
		m.sourceFilter = m.sourceChoices[nextIndex]
		m.message = fmt.Sprintf("Source filter: %s", m.sourceFilter)
	}
	m.rebuildRows()
}

func (m *Model) cycleGroupFilter() {
	if len(m.groupChoices) == 0 {
		m.groupFilter = allGroupsFilter
		m.message = "No group filters available."
		return
	}

	nextIndex := -1
	if m.groupFilter == allGroupsFilter {
		nextIndex = 0
	} else {
		for i, group := range m.groupChoices {
			if group == m.groupFilter {
				nextIndex = i + 1
				break
			}
		}
	}

	if nextIndex < 0 || nextIndex >= len(m.groupChoices) {
		m.groupFilter = allGroupsFilter
		m.message = "Group filter: all"
	} else {
		m.groupFilter = m.groupChoices[nextIndex]
		m.message = fmt.Sprintf("Group filter: %s", m.groupFilter)
	}
	m.rebuildRows()
}

func skillForTool(row domain.SkillRow, tool domain.Tool) *domain.ToolSkill {
	return staging.SkillForTool(row, tool)
}

func rowMatchesTextFilter(row domain.SkillRow, filter string) bool {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return true
	}

	values := []string{row.Name, row.Description}
	if row.Claude != nil {
		values = append(values, row.Claude.DisplayName, row.Claude.Description)
	}
	if row.Codex != nil {
		values = append(values, row.Codex.DisplayName, row.Codex.Description)
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), filter) {
			return true
		}
	}
	return false
}

func rowMatchesSourceFilter(row domain.SkillRow, filter domain.SourceLabel) bool {
	if filter == allSourcesFilter {
		return true
	}
	return rowHasSource(row, filter)
}

func rowHasSource(row domain.SkillRow, source domain.SourceLabel) bool {
	if row.Source == source {
		return true
	}
	if row.Claude != nil && row.Claude.Source == source {
		return true
	}
	if row.Codex != nil && row.Codex.Source == source {
		return true
	}
	return false
}

func rowMatchesGroupFilter(row domain.SkillRow, filter domain.GroupLabel) bool {
	if filter == allGroupsFilter {
		return true
	}
	return rowHasGroup(row, filter)
}

func rowHasGroup(row domain.SkillRow, group domain.GroupLabel) bool {
	return normalizedGroup(row.Group) == group
}

func normalizedGroup(group domain.GroupLabel) domain.GroupLabel {
	if group == "" {
		return domain.GroupUnknown
	}
	return group
}

func collectSourceChoices(rows []domain.SkillRow) []domain.SourceLabel {
	seen := make(map[domain.SourceLabel]struct{})
	for _, row := range rows {
		addSourceChoice(seen, row.Source)
		if row.Claude != nil {
			addSourceChoice(seen, row.Claude.Source)
		}
		if row.Codex != nil {
			addSourceChoice(seen, row.Codex.Source)
		}
	}

	choices := make([]domain.SourceLabel, 0, len(seen))
	for source := range seen {
		choices = append(choices, source)
	}
	sort.SliceStable(choices, func(i, j int) bool {
		return choices[i].String() < choices[j].String()
	})
	return choices
}

func addSourceChoice(seen map[domain.SourceLabel]struct{}, source domain.SourceLabel) {
	if source == "" {
		return
	}
	seen[source] = struct{}{}
}

func sourceChoiceAvailable(choices []domain.SourceLabel, source domain.SourceLabel) bool {
	for _, choice := range choices {
		if choice == source {
			return true
		}
	}
	return false
}

func collectGroupChoices(rows []domain.SkillRow) []domain.GroupLabel {
	seen := make(map[domain.GroupLabel]struct{})
	for _, row := range rows {
		addGroupChoice(seen, row.Group)
	}

	choices := make([]domain.GroupLabel, 0, len(seen))
	for group := range seen {
		choices = append(choices, group)
	}
	sort.SliceStable(choices, func(i, j int) bool {
		return choices[i].String() < choices[j].String()
	})
	return choices
}

func addGroupChoice(seen map[domain.GroupLabel]struct{}, group domain.GroupLabel) {
	if group == "" {
		return
	}
	seen[group] = struct{}{}
}

func groupChoiceAvailable(choices []domain.GroupLabel, group domain.GroupLabel) bool {
	for _, choice := range choices {
		if choice == group {
			return true
		}
	}
	return false
}
