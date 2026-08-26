package advisor

import (
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
)

func TestSearchRanksTaskSpecificMetadataAheadOfLooseMatches(t *testing.T) {
	rows := []model.SkillRow{
		searchTestRow("generic-docs", "Looks up official documentation.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOff),
		searchTestRow("quality-review", "Conducts multi-axis code review and quality review.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
		searchTestRow("decision-records", "Records architecture decisions and documentation as ADRs.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOff),
		searchTestRow("unrelated", "Builds release archives.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
	}

	results, err := Search(rows, SearchOptions{
		Tool: model.ToolCodex, Query: "documentation ADR decision record code review quality review", Limit: DefaultSearchLimit,
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 || !searchResultsContain(results[:2], "decision-records") || !searchResultsContain(results[:2], "quality-review") {
		t.Fatalf("ranked results = %v, want decision-records and quality-review first", searchResultNames(results))
	}
	if searchResultsContain(results, "unrelated") {
		t.Fatalf("ranked results = %v, want unrelated excluded", searchResultNames(results))
	}
}

func TestSearchWeightsNameAndExactPhrase(t *testing.T) {
	rows := []model.SkillRow{
		searchTestRow("release-helper", "An API appears in a long description with interface and design separated.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
		searchTestRow("api-interface-design", "Defines stable boundaries.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOff),
	}

	results, err := Search(rows, SearchOptions{Tool: model.ToolCodex, Query: "API interface design", Limit: 10})

	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Name != "api-interface-design" {
		t.Fatalf("ranked results = %v", searchResultNames(results))
	}
}

func TestSearchAppliesFieldWeightOrder(t *testing.T) {
	rows := []model.SkillRow{
		searchTestRow("quality-name", "Common helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
		searchTestRow("description-match", "Quality helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
		searchTestRow("group-match", "Common helper.", model.GroupLabel("quality"), model.SourceLocal, model.ToolCodex, model.SkillStateOn),
		searchTestRow("source-match", "Common helper.", model.GroupLocal, model.SourceLabel("quality"), model.ToolCodex, model.SkillStateOn),
	}

	results, err := Search(rows, SearchOptions{Tool: model.ToolCodex, Query: "quality", Limit: 10})

	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(searchResultNames(results), ","); got != "quality-name,description-match,group-match,source-match" {
		t.Fatalf("field-weighted order = %q", got)
	}
}

func TestSearchDoesNotAddFuzzyScoreWhenExactTokenMatches(t *testing.T) {
	rows := []model.SkillRow{
		searchTestRow("quality-review", "Review and reveiw helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
	}
	corpus := buildSearchCorpus(rows, model.ToolCodex)
	document := &corpus.documents[0]

	if got, want := corpus.scoreDocument(document, []string{"review"}), corpus.termScore(document, "review"); got != want {
		t.Fatalf("exact-precedence score = %f, want %f", got, want)
	}
}

func TestSearchNormalizesPunctuationPluralsAndTypos(t *testing.T) {
	rows := []model.SkillRow{
		searchTestRow("decision-note", "Maintains documentation and ADRs.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOff),
		searchTestRow("quality-check", "Performs quality reviews.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
	}

	results, err := Search(rows, SearchOptions{Tool: model.ToolCodex, Query: "documntation, ADR; quality reveiw", Limit: 10})

	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || !searchResultsContain(results, "decision-note") || !searchResultsContain(results, "quality-check") {
		t.Fatalf("ranked results = %v", searchResultNames(results))
	}
}

func TestSearchDoesNotFuzzyMatchShortTokens(t *testing.T) {
	rows := []model.SkillRow{
		searchTestRow("api-design", "Stable interface contracts.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
		searchTestRow("app-design", "Application layout contracts.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
	}

	results, err := Search(rows, SearchOptions{Tool: model.ToolCodex, Query: "api", Limit: 10})

	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "api-design" {
		t.Fatalf("ranked results = %v", searchResultNames(results))
	}
}

func TestSearchFiltersByRequestedToggleableToolCell(t *testing.T) {
	rows := []model.SkillRow{
		searchTestRow("codex-on", "Search helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
		searchTestRow("codex-off", "Search helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOff),
		searchTestRow("claude-only", "Search helper.", model.GroupLocal, model.SourceLocal, model.ToolClaude, model.SkillStateOn),
		searchTestRow("conflict", "Search helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateConflict),
		searchTestReadOnlyRow("system", "Search helper."),
	}

	results, err := Search(rows, SearchOptions{Tool: model.ToolCodex, Query: "search", Limit: 10})

	if err != nil {
		t.Fatal(err)
	}
	if got := searchResultNames(results); strings.Join(got, ",") != "codex-off,codex-on" {
		t.Fatalf("ranked results = %v", got)
	}
}

func TestSearchUsesDeterministicNameTieBreakAndLimit(t *testing.T) {
	rows := []model.SkillRow{
		searchTestRow("charlie", "Common helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
		searchTestRow("alpha", "Common helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
		searchTestRow("bravo", "Common helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn),
	}

	results, err := Search(rows, SearchOptions{Tool: model.ToolCodex, Query: "common", Limit: 2})

	if err != nil {
		t.Fatal(err)
	}
	if got := searchResultNames(results); strings.Join(got, ",") != "alpha,bravo" {
		t.Fatalf("ranked results = %v", got)
	}
}

func TestSearchValidatesBoundedInput(t *testing.T) {
	validRows := []model.SkillRow{searchTestRow("alpha", "Search helper.", model.GroupLocal, model.SourceLocal, model.ToolCodex, model.SkillStateOn)}
	tests := []struct {
		name    string
		options SearchOptions
		want    string
	}{
		{name: "tool", options: SearchOptions{Tool: "other", Query: "search", Limit: 1}, want: "unsupported tool"},
		{name: "empty", options: SearchOptions{Tool: model.ToolCodex, Query: " ", Limit: 1}, want: "non-empty"},
		{name: "characters", options: SearchOptions{Tool: model.ToolCodex, Query: strings.Repeat("x", MaxSearchQueryRunes+1), Limit: 1}, want: "characters"},
		{name: "tokens", options: SearchOptions{Tool: model.ToolCodex, Query: strings.Repeat("term ", MaxSearchQueryTokens+1), Limit: 1}, want: "tokens"},
		{name: "stop words", options: SearchOptions{Tool: model.ToolCodex, Query: "the and with", Limit: 1}, want: "searchable token"},
		{name: "limit low", options: SearchOptions{Tool: model.ToolCodex, Query: "search", Limit: 0}, want: "limit"},
		{name: "limit high", options: SearchOptions{Tool: model.ToolCodex, Query: "search", Limit: MaxSearchLimit + 1}, want: "limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Search(validRows, test.options); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Search() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestDamerauLevenshteinCountsAdjacentTranspositionOnce(t *testing.T) {
	if got := damerauLevenshtein("review", "reveiw"); got != 1 {
		t.Fatalf("distance = %d, want 1", got)
	}
}

func searchTestRow(name, description string, group model.GroupLabel, source model.SourceLabel, tool model.Tool, state model.SkillState) model.SkillRow {
	cell := &model.ToolSkill{Tool: tool, Name: name, Description: description, Group: group, Source: source, State: state}
	row := model.SkillRow{Name: name, Description: description, Group: group, Source: source}
	if tool == model.ToolClaude {
		row.Claude = cell
	} else {
		row.Codex = cell
	}
	return row
}

func searchTestReadOnlyRow(name, description string) model.SkillRow {
	cell := &model.ToolSkill{Tool: model.ToolCodex, Name: name, Description: description, Group: model.GroupCodexSystem, Source: model.SourceCodexSystem, State: model.SkillStateReadOnly, ReadOnly: true}
	return model.SkillRow{Name: name, Description: description, Group: model.GroupCodexSystem, Source: model.SourceCodexSystem, Codex: cell}
}

func searchResultNames(rows []model.SkillRow) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return names
}

func searchResultsContain(rows []model.SkillRow, name string) bool {
	for _, row := range rows {
		if row.Name == name {
			return true
		}
	}
	return false
}
