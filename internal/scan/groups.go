package scan

import (
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
)

// GroupSummaries returns sorted per-group summaries for already grouped rows.
func GroupSummaries(rows []model.SkillRow) []model.GroupSummary {
	byGroup := make(map[model.GroupLabel]*model.GroupSummary)
	sourceSets := make(map[model.GroupLabel]map[model.SourceLabel]struct{})

	for i := range rows {
		row := rows[i]
		group := row.Group
		if group == "" {
			group = model.GroupUnknown
		}

		summary := byGroup[group]
		if summary == nil {
			summary = &model.GroupSummary{Group: group}
			byGroup[group] = summary
			sourceSets[group] = make(map[model.SourceLabel]struct{})
		}
		summary.Rows++

		countCell(row.Claude, &summary.Claude, sourceSets[group])
		countCell(row.Codex, &summary.Codex, sourceSets[group])
		countCell(row.Muse, &summary.Muse, sourceSets[group])
	}

	summaries := make([]model.GroupSummary, 0, len(byGroup))
	for group, summary := range byGroup {
		sources := sortedSources(sourceSets[group])
		summary.Sources = sources
		summary.SourceText = sourceText(sources)
		summaries = append(summaries, *summary)
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].Group.String() < summaries[j].Group.String()
	})
	return summaries
}

func countCell(skill *model.ToolSkill, counts *model.ToolStateCounts, sources map[model.SourceLabel]struct{}) {
	if skill == nil {
		return
	}

	if skill.Source != "" {
		sources[skill.Source] = struct{}{}
	}

	switch skill.State {
	case model.SkillStateOn:
		counts.On++
	case model.SkillStateOff:
		counts.Off++
	case model.SkillStateConflict:
		counts.Conflict++
	case model.SkillStateReadOnly:
		counts.ReadOnly++
	}
}

func sortedSources(sourceSet map[model.SourceLabel]struct{}) []model.SourceLabel {
	sources := make([]model.SourceLabel, 0, len(sourceSet))
	for source := range sourceSet {
		if source != "" {
			sources = append(sources, source)
		}
	}
	sort.SliceStable(sources, func(i, j int) bool {
		return sources[i].String() < sources[j].String()
	})
	return sources
}

func sourceText(sources []model.SourceLabel) string {
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		parts = append(parts, source.String())
	}
	return strings.Join(parts, ", ")
}
