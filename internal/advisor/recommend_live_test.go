package advisor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/typesafe"
)

func TestLiveRecommendationEvaluation(t *testing.T) {
	if os.Getenv("SKILL_MANAGER_LIVE_TYPESAFE") != "1" || os.Getenv("TYPESAFE_API_KEY") == "" {
		t.Skip("set SKILL_MANAGER_LIVE_TYPESAFE=1 and TYPESAFE_API_KEY to measure live quality")
	}
	matches, err := filepath.Glob(filepath.Join("testdata", "recommendation", "cases", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	client := typesafe.New(typesafe.Key(os.Getenv("TYPESAFE_API_KEY")))
	var selectionErrors, usefulHits, usefulTotal, falseSelections, retrievalHits, retrievalTotal int
	var inputTokens, outputTokens, requests int
	started := time.Now()
	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err != nil {
			t.Fatal(err)
		}
		var fixture recommendationFixture
		if err := json.Unmarshal(data, &fixture); err != nil {
			t.Fatal(err)
		}
		rows := make([]model.SkillRow, 0, len(fixture.Catalog))
		tool, ok := model.ParseTool(fixture.Tool)
		if !ok {
			tool = model.ToolCodex
		}
		for _, item := range fixture.Catalog {
			state := model.SkillStateOn
			if item.State == "off" {
				state = model.SkillStateOff
			}
			rows = append(rows, recommendRow(item.Name, item.Description, tool, state))
		}
		result, err := Recommend(context.Background(), rows, RecommendOptions{
			Tool:              tool,
			Query:             fixture.Query,
			TaskBrief:         fixture.Task,
			RequestedProvider: ProviderTypeSafe,
			NewProvider:       func(context.Context) (Provider, error) { return client, nil },
		})
		if err != nil {
			t.Logf("%s error: %v", fixture.ID, err)
			continue
		}
		if result.Usage != nil {
			inputTokens += result.Usage.InputTokens
			outputTokens += result.Usage.OutputTokens
			requests += result.Usage.Requests
		}
		got := map[string]struct{}{}
		for _, name := range result.RecommendedSkills {
			got[name] = struct{}{}
		}
		if fixture.Expected.NoMatch {
			if len(result.RecommendedSkills) > 0 {
				falseSelections++
				selectionErrors++
			}
			continue
		}
		usefulTotal++
		hit := true
		for _, name := range fixture.Expected.Required {
			retrievalTotal++
			if _, ok := got[name]; ok {
				retrievalHits++
			} else {
				hit = false
			}
		}
		if hit && len(fixture.Expected.Required) > 0 {
			usefulHits++
		} else {
			selectionErrors++
		}
		t.Logf("%s split=%s outcome=%s skills=%v", fixture.ID, fixture.Split, result.Outcome, result.RecommendedSkills)
	}
	t.Logf("selection_errors=%d useful_recall=%d/%d false_selections=%d retrieval_recall=%d/%d requests=%d input_tokens=%d output_tokens=%d latency=%s",
		selectionErrors, usefulHits, usefulTotal, falseSelections, retrievalHits, retrievalTotal, requests, inputTokens, outputTokens, time.Since(started))
}
