package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	domain "github.com/dees91/agent-skill-manager/internal/model"
)

const (
	maxSkillColumnWidth = 44
	minSkillColumnWidth = 24
	minGroupWidth       = 12
	stateColumnWidth    = 11
)

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	subtleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	activeHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("63"))
	cursorStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	nameStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	activeCellStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("63"))
	pendingCellStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	conflictCellStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	readOnlyCellStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	messageStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	if m.err != nil {
		fmt.Fprintf(&b, "Error: %v\n\n", m.err)
	}

	b.WriteString(titleStyle.Render("Skill Manager"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf("Active column: %s | Rows: %d | Tab switches Claude/Codex/Muse/Grok", toolTitle(m.activeTool), len(m.rows))))
	b.WriteString("\n\n")

	skillWidth := skillColumnWidth(m.width)
	groupWidth := groupColumnWidth(m.width, skillWidth)

	b.WriteString(formatHeader(m.activeTool, skillWidth))
	b.WriteString("\n")
	if len(m.rows) == 0 {
		b.WriteString("No skills found.\n")
	} else {
		start, end := m.visibleRange()
		for i := start; i < end; i++ {
			row := m.rows[i]
			cursor := "  "
			if i == m.cursor {
				cursor = ">>"
			}
			line := fmt.Sprintf(
				"%s %s %s %s %s %s %s",
				cursorStyle.Width(2).Render(cursor),
				nameStyle.Width(skillWidth).Render(truncate(row.Name, skillWidth)),
				formatCell(domain.ToolClaude, row.Name, row.Claude, m.activeTool, i == m.cursor, m.pending),
				formatCell(domain.ToolCodex, row.Name, row.Codex, m.activeTool, i == m.cursor, m.pending),
				formatCell(domain.ToolMuse, row.Name, row.Muse, m.activeTool, i == m.cursor, m.pending),
				formatCell(domain.ToolGrok, row.Name, row.Grok, m.activeTool, i == m.cursor, m.pending),
				truncate(row.Group.String(), groupWidth),
			)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if m.message != "" {
		fmt.Fprintf(&b, "\n%s\n", messageStyle.Render(m.message))
	}

	if m.showDetails {
		b.WriteString("\n")
		m.writeDetails(&b)
	}

	fmt.Fprintf(&b, "\n%s\n", subtleStyle.Render(m.filterStatusLine()))
	b.WriteString(helpStyle.Render("Tab switch tool | Space toggle | b row | g group | A all | a/Enter apply | u undo | U clear | o read-only | / filter | s source | G filter | d details | r rescan | q quit"))
	b.WriteString("\n")
	return b.String()
}

func (m Model) visibleRange() (int, int) {
	if len(m.rows) == 0 {
		return 0, 0
	}

	visibleRows := m.maxVisibleRows()
	if visibleRows >= len(m.rows) {
		return 0, len(m.rows)
	}

	start := m.cursor - visibleRows/2
	if start < 0 {
		start = 0
	}
	if start+visibleRows > len(m.rows) {
		start = len(m.rows) - visibleRows
	}
	return start, start + visibleRows
}

func (m Model) maxVisibleRows() int {
	if m.height <= 0 {
		return 18
	}

	reserved := 7
	if m.err != nil {
		reserved += 2
	}
	if m.message != "" {
		reserved += 2
	}
	if m.showDetails {
		reserved += 17
	}

	rows := m.height - reserved
	if rows < 1 {
		return 1
	}
	return rows
}

func formatCell(tool domain.Tool, skillName string, skill *domain.ToolSkill, activeTool domain.Tool, selected bool, pending map[pendingKey]domain.OperationKind) string {
	state := domain.SkillStateMissing.String()
	if skill != nil {
		state = skill.State.String()
	}
	if kind, ok := pending[pendingKey{tool: tool, skillName: skillName}]; ok {
		state = pendingStateLabel(kind)
	}

	style := lipgloss.NewStyle()
	if skill != nil {
		switch {
		case skill.State == domain.SkillStateConflict:
			style = conflictCellStyle
		case skill.ReadOnly || skill.State == domain.SkillStateReadOnly:
			style = readOnlyCellStyle
		}
	}
	if _, ok := pending[pendingKey{tool: tool, skillName: skillName}]; ok {
		style = pendingCellStyle
	}
	if selected && tool == activeTool {
		return activeCellStyle.Width(stateColumnWidth).Render("[" + state + "]")
	}
	return style.Width(stateColumnWidth).Render(state)
}

func formatHeader(activeTool domain.Tool, skillWidth int) string {
	claude := headerCell("Claude", activeTool == domain.ToolClaude)
	codex := headerCell("Codex", activeTool == domain.ToolCodex)
	muse := headerCell("Muse", activeTool == domain.ToolMuse)
	grok := headerCell("Grok", activeTool == domain.ToolGrok)
	return fmt.Sprintf(
		"%s %s %s %s %s %s %s",
		headerStyle.Width(2).Render(""),
		headerStyle.Width(skillWidth).Render("Skill"),
		claude,
		codex,
		muse,
		grok,
		headerStyle.Render("Group"),
	)
}

func headerCell(label string, active bool) string {
	if active {
		return activeHeaderStyle.Width(stateColumnWidth).Render("[" + label + "]")
	}
	return headerStyle.Width(stateColumnWidth).Render(label)
}

func toolTitle(tool domain.Tool) string {
	switch tool {
	case domain.ToolClaude:
		return "Claude"
	case domain.ToolCodex:
		return "Codex"
	case domain.ToolMuse:
		return "Muse"
	case domain.ToolGrok:
		return "Grok"
	default:
		return tool.String()
	}
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func pendingStateLabel(kind domain.OperationKind) string {
	switch kind {
	case domain.OperationDisable:
		return "ON->OFF"
	case domain.OperationEnable:
		return "OFF->ON"
	default:
		return domain.SkillStatePending.String()
	}
}

func (m Model) filterStatusLine() string {
	parts := []string{}
	if m.showReadOnly {
		parts = append(parts, "Read-only: shown")
	} else {
		parts = append(parts, "Read-only: hidden")
	}

	if m.editingFilter {
		parts = append(parts, "Filter: /"+m.textFilter)
	} else if m.textFilter != "" {
		parts = append(parts, "Filter: "+m.textFilter)
	} else {
		parts = append(parts, "Filter: all")
	}

	if m.sourceFilter == allSourcesFilter {
		parts = append(parts, "Source: all")
	} else {
		parts = append(parts, "Source: "+m.sourceFilter.String())
	}

	if m.groupFilter == allGroupsFilter {
		parts = append(parts, "Group: all")
	} else {
		parts = append(parts, "Group: "+m.groupFilter.String())
	}

	if len(m.pending) > 0 {
		parts = append(parts, fmt.Sprintf("Pending: %d", len(m.pending)))
	}
	if len(m.rows) > 0 {
		start, end := m.visibleRange()
		if start > 0 || end < len(m.rows) {
			parts = append(parts, fmt.Sprintf("Rows: %d-%d/%d", start+1, end, len(m.rows)))
		}
	}

	return strings.Join(parts, " | ")
}

func skillColumnWidth(totalWidth int) int {
	if totalWidth <= 0 {
		return maxSkillColumnWidth
	}

	width := totalWidth - fixedTableWidthWithoutSkill() - minGroupWidth
	if width < minSkillColumnWidth {
		return minSkillColumnWidth
	}
	if width > maxSkillColumnWidth {
		return maxSkillColumnWidth
	}
	return width
}

func groupColumnWidth(totalWidth, skillWidth int) int {
	if totalWidth <= 0 {
		return 20
	}
	width := totalWidth - fixedTableWidthWithoutSkill() - skillWidth
	if width < minGroupWidth {
		return minGroupWidth
	}
	return width
}

// fixedTableWidthWithoutSkill covers the cursor column (2), one separator per
// column gap (skill, tools, and group make len(Tools())+2 gaps), and one fixed
// state column per tool. Derive it from len(Tools()) so adding a tool cannot
// silently skew the Skill and Group columns.
func fixedTableWidthWithoutSkill() int {
	tools := len(domain.Tools())
	return 2 + (tools + 2) + tools*stateColumnWidth
}

func (m Model) writeDetails(b *strings.Builder) {
	row, ok := m.selectedRow()
	if !ok {
		b.WriteString("Details\n")
		b.WriteString("No selected skill.\n")
		return
	}

	b.WriteString("Details\n")
	fmt.Fprintf(b, "Skill: %s\n", row.Name)
	if description := rowDescription(row); description != "" {
		fmt.Fprintf(b, "Description: %s\n", description)
	}
	fmt.Fprintf(b, "Group: %s\n", row.Group)
	fmt.Fprintf(b, "Source: %s\n", row.Source)
	writeToolDetails(b, "Claude", domain.ToolClaude, row.Name, row.Claude, m.pending)
	writeToolDetails(b, "Codex", domain.ToolCodex, row.Name, row.Codex, m.pending)
	writeToolDetails(b, "Muse", domain.ToolMuse, row.Name, row.Muse, m.pending)
	writeToolDetails(b, "Grok", domain.ToolGrok, row.Name, row.Grok, m.pending)
}

func rowDescription(row domain.SkillRow) string {
	if row.Description != "" {
		return row.Description
	}
	if row.Claude != nil && row.Claude.Description != "" {
		return row.Claude.Description
	}
	if row.Codex != nil && row.Codex.Description != "" {
		return row.Codex.Description
	}
	if row.Muse != nil && row.Muse.Description != "" {
		return row.Muse.Description
	}
	if row.Grok != nil && row.Grok.Description != "" {
		return row.Grok.Description
	}
	return ""
}

func writeToolDetails(b *strings.Builder, label string, tool domain.Tool, skillName string, skill *domain.ToolSkill, pending map[pendingKey]domain.OperationKind) {
	fmt.Fprintf(b, "%s:\n", label)
	if skill == nil {
		b.WriteString("  not present\n")
		return
	}

	fmt.Fprintf(b, "  State: %s\n", skill.State)
	if kind, ok := pending[pendingKey{tool: tool, skillName: skillName}]; ok {
		fmt.Fprintf(b, "  Pending: %s\n", kind)
	}
	fmt.Fprintf(b, "  Group: %s\n", skill.Group)
	fmt.Fprintf(b, "  Source: %s\n", skill.Source)
	if skill.EntryType != "" {
		fmt.Fprintf(b, "  Entry type: %s\n", skill.EntryType)
	}
	writeOptionalDetail(b, "Active path", skill.ActivePath)
	writeOptionalDetail(b, "Disabled path", skill.DisabledPath)
	writeOptionalDetail(b, "Skill file", skill.SkillFilePath)
	writeOptionalDetail(b, "Symlink target", skill.SymlinkTarget)
	writeOptionalDetail(b, "Repo origin", skill.RepoOrigin)
	writeOptionalDetail(b, "Repo commit", skill.RepoCommit)
	if skill.Conflict != nil {
		b.WriteString("  Conflict:\n")
		writeOptionalDetailIndented(b, "Original path", skill.Conflict.OriginalPath, "    ")
		writeOptionalDetailIndented(b, "Disabled path", skill.Conflict.DisabledPath, "    ")
		writeOptionalDetailIndented(b, "Blocker path", skill.Conflict.BlockerPath, "    ")
		writeOptionalDetailIndented(b, "Message", skill.Conflict.Message, "    ")
	}
}

func writeOptionalDetail(b *strings.Builder, label, value string) {
	writeOptionalDetailIndented(b, label, value, "  ")
}

func writeOptionalDetailIndented(b *strings.Builder, label, value, indent string) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s%s: %s\n", indent, label, value)
}
