package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/typesafe"
)

func TestRecommendLocalModeDoesNotCallFactory(t *testing.T) {
	rows := []model.SkillRow{recommendRow("alpha", "video helper", model.ToolCodex, model.SkillStateOn)}
	called := false
	result, err := Recommend(context.Background(), rows, RecommendOptions{
		Tool:              model.ToolCodex,
		Query:             "video helper",
		TaskBrief:         "Render a video clip with ffmpeg.",
		RequestedProvider: ProviderLocal,
		NewProvider: func(context.Context) (Provider, error) {
			called = true
			return nil, errors.New("factory")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called || result.Outcome != OutcomeLocalCandidates || result.FallbackReason != "" || result.UsedProvider != ProviderLocal {
		t.Fatalf("result = %#v called=%v", result, called)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Name != "alpha" {
		t.Fatalf("candidates = %#v", result.Candidates)
	}
}

func TestRecommendEmptyCatalogSkipsFactory(t *testing.T) {
	called := false
	result, err := Recommend(context.Background(), []model.SkillRow{searchTestReadOnlyRow("imagegen", "system imagegen")}, RecommendOptions{
		Tool:              model.ToolCodex,
		Query:             "imagegen helper",
		TaskBrief:         "Generate an image.",
		RequestedProvider: ProviderTypeSafe,
		NewProvider: func(context.Context) (Provider, error) {
			called = true
			return nil, errors.New("factory")
		},
	})
	if err != nil || called || result.FallbackReason != FallbackNoCandidates {
		t.Fatalf("result = %#v called=%v err=%v", result, called, err)
	}
}

func TestRecommendExcludesIneligibleRowsFromCatalog(t *testing.T) {
	rows := []model.SkillRow{
		recommendRow("keep-me", "video encoder helper", model.ToolCodex, model.SkillStateOn),
		recommendRow("skill-advisor", "advisor helper", model.ToolCodex, model.SkillStateOn),
		searchTestReadOnlyRow("imagegen", "system imagegen"),
		recommendRow("other-host", "video encoder helper", model.ToolClaude, model.SkillStateOn),
		recommendRow("missing-one", "video encoder helper", model.ToolCodex, model.SkillStateMissing),
	}
	fake := &scriptedProvider{handler: highNouls}
	result, err := Recommend(context.Background(), rows, RecommendOptions{
		Tool:              model.ToolCodex,
		Query:             "video encoder",
		TaskBrief:         "Encode a video file.",
		RequestedProvider: ProviderTypeSafe,
		NewProvider:       func(context.Context) (Provider, error) { return fake, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CatalogSize != 1 || result.Outcome != OutcomeRecommended {
		t.Fatalf("result = %#v", result)
	}
	state0, _ := fake.requests[0].State.(stage1State)
	if len(state0.Catalog) != 1 || state0.Catalog[0].Name != "keep-me" {
		t.Fatalf("catalog = %#v", state0.Catalog)
	}
	raw, _ := json.Marshal(state0.Catalog[0])
	if strings.Contains(string(raw), "group") || strings.Contains(string(raw), "source") || strings.Contains(string(raw), "/") {
		t.Fatalf("catalog leaked fields: %s", raw)
	}
}

func TestRecommendGateNoneAndFloorNone(t *testing.T) {
	rows := []model.SkillRow{recommendRow("alpha", "video helper", model.ToolCodex, model.SkillStateOn)}
	t.Run("gate", func(t *testing.T) {
		fake := &scriptedProvider{handler: func(req typesafe.Request) (typesafe.Response, error) {
			return noulResponse(req, map[string]float64{"gate": 0.1}), nil
		}}
		result, err := Recommend(context.Background(), rows, typesafeOptions(fake))
		if err != nil || result.Outcome != OutcomeNone || result.Usage.Requests != 1 || len(result.RecommendedSkills) != 0 {
			t.Fatalf("result = %#v err=%v", result, err)
		}
	})
	t.Run("floor", func(t *testing.T) {
		fake := &scriptedProvider{handler: func(req typesafe.Request) (typesafe.Response, error) {
			return noulResponse(req, map[string]float64{"gate": 0.9, "s0": 0.1}), nil
		}}
		result, err := Recommend(context.Background(), rows, typesafeOptions(fake))
		if err != nil || result.Outcome != OutcomeNone || result.Usage.Requests != 1 {
			t.Fatalf("result = %#v err=%v", result, err)
		}
	})
}

func TestRecommendStage2FiltersAndOrders(t *testing.T) {
	rows := []model.SkillRow{
		recommendRow("alpha", "video helper", model.ToolCodex, model.SkillStateOn),
		recommendRow("beta", "video helper", model.ToolCodex, model.SkillStateOff),
		recommendRow("gamma", "video helper", model.ToolCodex, model.SkillStateOn),
	}
	fake := &scriptedProvider{handler: func(req typesafe.Request) (typesafe.Response, error) {
		values := map[string]float64{"gate": 0.9, "s0": 0.8, "s1": 0.7, "s2": 0.6, "r0": 0.9, "n0": 0.6, "r1": 0.9, "n1": 0.9, "r2": 0.2, "n2": 0.9}
		return noulResponse(req, values), nil
	}}
	result, err := Recommend(context.Background(), rows, typesafeOptions(fake))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeRecommended || result.Usage.Requests != 2 {
		t.Fatalf("result = %#v", result)
	}
	if got := strings.Join(result.RecommendedSkills, ","); got != "beta,alpha" {
		t.Fatalf("recommended = %v", result.RecommendedSkills)
	}
	if !candidateNamesContain(result.Candidates, "alpha") || !candidateNamesContain(result.Candidates, "beta") {
		t.Fatalf("candidates missing recommended names: %v", searchResultNames(result.Candidates))
	}
}

func TestRecommendBM25FMissStillReachesJev(t *testing.T) {
	rows := make([]model.SkillRow, 0, 22)
	for i := 0; i < 21; i++ {
		rows = append(rows, recommendRow(fmt.Sprintf("video-%02d", i), "video encoder helper", model.ToolCodex, model.SkillStateOn))
	}
	rows = append(rows, recommendRow("hidden-zip", "archive compression format specialist", model.ToolCodex, model.SkillStateOff))
	fake := &scriptedProvider{handler: func(req typesafe.Request) (typesafe.Response, error) {
		values := map[string]float64{"gate": 0.9}
		if state, ok := req.State.(stage1State); ok {
			for _, skill := range state.Catalog {
				score := 0.1
				if skill.Name == "hidden-zip" {
					score = 0.95
				}
				values[fmt.Sprintf("s%d", skill.Index)] = score
			}
		}
		if state, ok := req.State.(stage2State); ok {
			for i, skill := range state.Shortlist {
				score := 0.1
				if skill.Name == "hidden-zip" {
					score = 0.95
				}
				values[fmt.Sprintf("r%d", i)] = score
				values[fmt.Sprintf("n%d", i)] = score
			}
		}
		return noulResponse(req, values), nil
	}}
	result, err := Recommend(context.Background(), rows, RecommendOptions{
		Tool:              model.ToolCodex,
		Query:             "video encoder helper",
		TaskBrief:         "Compress a zip archive.",
		RequestedProvider: ProviderTypeSafe,
		NewProvider:       func(context.Context) (Provider, error) { return fake, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeRecommended || len(result.RecommendedSkills) != 1 || result.RecommendedSkills[0] != "hidden-zip" {
		t.Fatalf("result = %#v", result)
	}
	localNames := searchResultNames(result.Candidates)
	if containsName(localNames[:min(20, len(localNames))], "hidden-zip") && len(result.Candidates) == 1 {
		t.Fatalf("expected hidden-zip to be outside the BM25F-only view: %v", localNames)
	}
}

func TestRecommendChunkFailureCancelsAndSkipsStage2(t *testing.T) {
	rows := manyRecommendRows(250, model.ToolCodex)
	var stage2 atomic.Int32
	fake := &scriptedProvider{delay: 40 * time.Millisecond, handler: func(req typesafe.Request) (typesafe.Response, error) {
		if _, ok := req.State.(stage2State); ok {
			stage2.Add(1)
			return typesafe.Response{}, errors.New("stage2")
		}
		if _, ok := req.State.(stage1State); ok {
			if _, hasGate := req.Questions["gate"]; !hasGate {
				return typesafe.Response{}, &typesafe.Error{Reason: typesafe.ReasonRateLimited, Status: 429}
			}
		}
		return highNouls(req)
	}}
	result, err := Recommend(context.Background(), rows, typesafeOptions(fake))
	if err != nil || result.Outcome != OutcomeLocalCandidates || result.FallbackReason != typesafe.ReasonRateLimited {
		t.Fatalf("result = %#v err=%v", result, err)
	}
	if stage2.Load() != 0 {
		t.Fatal("stage 2 ran")
	}
	if fake.maxInFlight.Load() > ChunkConcurrency {
		t.Fatalf("in flight = %d", fake.maxInFlight.Load())
	}
}

func TestRecommendMalformedIDsAndStaleRefresh(t *testing.T) {
	rows := []model.SkillRow{recommendRow("alpha", "video helper", model.ToolCodex, model.SkillStateOn)}
	t.Run("extra-id", func(t *testing.T) {
		fake := &scriptedProvider{handler: func(req typesafe.Request) (typesafe.Response, error) {
			resp := noulResponse(req, nil)
			v := 0.9
			resp.Answers["extra"] = typesafe.Answer{Type: "noul", Noul: &v}
			return resp, nil
		}}
		result, err := Recommend(context.Background(), rows, typesafeOptions(fake))
		if err != nil || result.FallbackReason != typesafe.ReasonMalformedResponse {
			t.Fatalf("result = %#v err=%v", result, err)
		}
	})
	t.Run("refresh", func(t *testing.T) {
		fake := &scriptedProvider{handler: highNouls}
		result, err := Recommend(context.Background(), rows, RecommendOptions{
			Tool:              model.ToolCodex,
			Query:             "video helper",
			TaskBrief:         "Render a video clip.",
			RequestedProvider: ProviderTypeSafe,
			NewProvider:       func(context.Context) (Provider, error) { return fake, nil },
			Refresh:           func() ([]model.SkillRow, error) { return nil, nil },
		})
		if err != nil || result.FallbackReason != FallbackCandidatesChanged {
			t.Fatalf("result = %#v err=%v", result, err)
		}
	})
}

func TestRecommendFallbackReasons(t *testing.T) {
	rows := []model.SkillRow{recommendRow("alpha", "video helper", model.ToolCodex, model.SkillStateOn)}
	cases := []struct {
		reason string
		err    error
	}{
		{FallbackMissingKey, &Fallback{Reason: FallbackMissingKey}},
		{FallbackStoreUnavailable, &Fallback{Reason: FallbackStoreUnavailable}},
		{FallbackStoreDenied, &Fallback{Reason: FallbackStoreDenied}},
		{typesafe.ReasonInvalidKey, &typesafe.Error{Reason: typesafe.ReasonInvalidKey, Status: 401}},
		{typesafe.ReasonRateLimited, &typesafe.Error{Reason: typesafe.ReasonRateLimited, Status: 429}},
	}
	for _, test := range cases {
		t.Run(test.reason, func(t *testing.T) {
			result, err := Recommend(context.Background(), rows, RecommendOptions{
				Tool:              model.ToolCodex,
				Query:             "video helper",
				TaskBrief:         "Render a video clip.",
				RequestedProvider: ProviderTypeSafe,
				NewProvider: func(context.Context) (Provider, error) {
					if _, ok := test.err.(*Fallback); ok {
						return nil, test.err
					}
					return &scriptedProvider{handler: func(typesafe.Request) (typesafe.Response, error) { return typesafe.Response{}, test.err }}, nil
				},
			})
			if err != nil || result.FallbackReason != test.reason || result.Outcome != OutcomeLocalCandidates {
				t.Fatalf("result = %#v err=%v", result, err)
			}
		})
	}
	unreadable, err := Recommend(context.Background(), rows, RecommendOptions{
		Tool:               model.ToolCodex,
		Query:              "video helper",
		TaskBrief:          "Render a video clip.",
		RequestedProvider:  ProviderTypeSafe,
		SettingsUnreadable: true,
		NewProvider:        func(context.Context) (Provider, error) { return nil, errors.New("no") },
	})
	if err != nil || unreadable.FallbackReason != FallbackSettingsUnreadable {
		t.Fatalf("unreadable = %#v err=%v", unreadable, err)
	}
}

func TestRecommendRejectsBriefBeforeScan(t *testing.T) {
	_, err := Recommend(context.Background(), nil, RecommendOptions{Tool: model.ToolCodex, Query: "video helper", TaskBrief: ""})
	if err == nil || strings.Contains(err.Error(), "Render") {
		t.Fatalf("err = %v", err)
	}
	_, err = Recommend(context.Background(), nil, RecommendOptions{Tool: model.ToolCodex, Query: "video helper", TaskBrief: "bad\x00brief"})
	if err == nil {
		t.Fatal("expected control character error")
	}
}

func TestRecommendDoesNotCreateAdvisorLock(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	rows := []model.SkillRow{recommendRow("alpha", "video helper", model.ToolCodex, model.SkillStateOn)}
	fake := &scriptedProvider{handler: highNouls}
	if _, err := Recommend(context.Background(), rows, typesafeOptions(fake)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.AdvisorLockFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file err = %v", err)
	}
}

func TestTruncateUTF8BytesRuneBoundary(t *testing.T) {
	value := strings.Repeat("é", 300)
	got := truncateUTF8Bytes(value, MaxCatalogDescriptionBytes)
	if len(got) > MaxCatalogDescriptionBytes || !utf8.ValidString(got) {
		t.Fatalf("len=%d valid=%v", len(got), utf8.ValidString(got))
	}
	got = truncateUTF8Bytes(value+value+value+value, MaxCandidateDescriptionBytes)
	if len(got) > MaxCandidateDescriptionBytes || !utf8.ValidString(got) {
		t.Fatalf("2048 len=%d valid=%v", len(got), utf8.ValidString(got))
	}
}

func TestChunkCatalogBoundaries(t *testing.T) {
	one := []catalogSkill{{Index: 0, Name: "a", Description: "short"}}
	chunks, err := chunkCatalog("task", one)
	if err != nil || len(chunks) != 1 {
		t.Fatalf("one = %d err=%v", len(chunks), err)
	}
	large := make([]catalogSkill, 0, 250)
	for i := 0; i < 250; i++ {
		large = append(large, catalogSkill{Index: i, Name: fmt.Sprintf("s%04d", i), Description: strings.Repeat("d", 400)})
	}
	chunks, err = chunkCatalog("task brief for encoding", large)
	if err != nil || len(chunks) < 2 {
		t.Fatalf("large chunks = %d err=%v", len(chunks), err)
	}
	seen := map[int]struct{}{}
	for _, chunk := range chunks {
		for _, skill := range chunk.skills {
			if _, ok := seen[skill.Index]; ok {
				t.Fatalf("duplicate index %d", skill.Index)
			}
			seen[skill.Index] = struct{}{}
		}
	}
	if len(seen) != 250 {
		t.Fatalf("seen = %d", len(seen))
	}
	tooMany := make([]catalogSkill, 0, MaxCatalogChunks*300)
	for i := 0; i < MaxCatalogChunks*300; i++ {
		tooMany = append(tooMany, catalogSkill{Index: i, Name: fmt.Sprintf("x%05d", i), Description: strings.Repeat("z", 500)})
	}
	if _, err := chunkCatalog("task", tooMany); err == nil {
		t.Fatal("expected catalog_too_large")
	}
}

func TestRecommendGateOnlyInChunkZero(t *testing.T) {
	rows := manyRecommendRows(250, model.ToolCodex)
	fake := &scriptedProvider{handler: highNouls}
	if _, err := Recommend(context.Background(), rows, typesafeOptions(fake)); err != nil {
		t.Fatal(err)
	}
	gateCount := 0
	stage1 := 0
	for _, req := range fake.requests {
		if _, ok := req.State.(stage1State); !ok {
			continue
		}
		stage1++
		if _, ok := req.Questions["gate"]; ok {
			gateCount++
		}
	}
	if stage1 < 2 {
		t.Fatalf("stage1 requests = %d", stage1)
	}
	if gateCount != 1 {
		t.Fatalf("gate questions = %d, want 1 across %d stage1 requests", gateCount, stage1)
	}
}

func TestRecommendFixtureCases(t *testing.T) {
	root := filepath.Join("testdata", "recommendation", "cases")
	matches, err := filepath.Glob(filepath.Join(root, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no fixture cases")
	}
	for _, match := range matches {
		t.Run(filepath.Base(match), func(t *testing.T) {
			data, err := os.ReadFile(match)
			if err != nil {
				t.Fatal(err)
			}
			var fixture recommendationFixture
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatal(err)
			}
			rows := make([]model.SkillRow, 0, len(fixture.Catalog))
			for _, item := range fixture.Catalog {
				state := model.SkillStateOn
				if item.State == "off" {
					state = model.SkillStateOff
				}
				tool := model.ToolCodex
				if fixture.Tool != "" {
					parsed, _ := model.ParseTool(fixture.Tool)
					tool = parsed
				}
				rows = append(rows, recommendRow(item.Name, item.Description, tool, state))
			}
			fake := &scriptedProvider{handler: fixtureHandler(fixture)}
			tool, _ := model.ParseTool(fixture.Tool)
			if tool == "" {
				tool = model.ToolCodex
			}
			result, err := Recommend(context.Background(), rows, RecommendOptions{
				Tool:              tool,
				Query:             fixture.Query,
				TaskBrief:         fixture.Task,
				RequestedProvider: ProviderTypeSafe,
				NewProvider:       func(context.Context) (Provider, error) { return fake, nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			if fixture.Expected.NoMatch {
				if result.Outcome != OutcomeNone {
					t.Fatalf("outcome = %s skills=%v", result.Outcome, result.RecommendedSkills)
				}
				return
			}
			got := map[string]struct{}{}
			for _, name := range result.RecommendedSkills {
				got[name] = struct{}{}
			}
			for _, name := range fixture.Expected.Required {
				if _, ok := got[name]; !ok {
					t.Fatalf("missing required %s in %v", name, result.RecommendedSkills)
				}
			}
			for _, name := range fixture.Expected.Excluded {
				if _, ok := got[name]; ok {
					t.Fatalf("excluded %s was recommended: %v", name, result.RecommendedSkills)
				}
			}
		})
	}
}

type recommendationFixture struct {
	ID       string `json:"id"`
	Split    string `json:"split"`
	Language string `json:"language"`
	Tool     string `json:"tool"`
	Query    string `json:"query"`
	Task     string `json:"task"`
	Catalog  []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		State       string `json:"state"`
	} `json:"catalog"`
	Expected struct {
		Required   []string `json:"required"`
		Acceptable []string `json:"acceptable"`
		Excluded   []string `json:"excluded"`
		NoMatch    bool     `json:"noMatch"`
	} `json:"expected"`
}

func fixtureHandler(fixture recommendationFixture) func(typesafe.Request) (typesafe.Response, error) {
	required := map[string]struct{}{}
	for _, name := range fixture.Expected.Required {
		required[name] = struct{}{}
	}
	acceptable := map[string]struct{}{}
	for _, name := range fixture.Expected.Acceptable {
		acceptable[name] = struct{}{}
	}
	excluded := map[string]struct{}{}
	for _, name := range fixture.Expected.Excluded {
		excluded[name] = struct{}{}
	}
	return func(req typesafe.Request) (typesafe.Response, error) {
		values := map[string]float64{"gate": 0.9}
		if fixture.Expected.NoMatch {
			values["gate"] = 0.1
		}
		scoreName := func(name string) float64 {
			if _, ok := excluded[name]; ok {
				return 0.05
			}
			if _, ok := required[name]; ok {
				return 0.95
			}
			if _, ok := acceptable[name]; ok {
				return 0.7
			}
			return 0.2
		}
		if state, ok := req.State.(stage1State); ok {
			for _, skill := range state.Catalog {
				values[fmt.Sprintf("s%d", skill.Index)] = scoreName(skill.Name)
			}
		}
		if state, ok := req.State.(stage2State); ok {
			for i, skill := range state.Shortlist {
				score := scoreName(skill.Name)
				values[fmt.Sprintf("r%d", i)] = score
				values[fmt.Sprintf("n%d", i)] = score
			}
		}
		return noulResponse(req, values), nil
	}
}

type scriptedProvider struct {
	mu          sync.Mutex
	requests    []typesafe.Request
	handler     func(typesafe.Request) (typesafe.Response, error)
	delay       time.Duration
	inFlight    atomic.Int32
	maxInFlight atomic.Int32
}

func (s *scriptedProvider) Evaluate(ctx context.Context, request typesafe.Request) (typesafe.Response, error) {
	n := s.inFlight.Add(1)
	defer s.inFlight.Add(-1)
	for {
		old := s.maxInFlight.Load()
		if n <= old || s.maxInFlight.CompareAndSwap(old, n) {
			break
		}
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return typesafe.Response{}, ctx.Err()
		}
	}
	s.mu.Lock()
	s.requests = append(s.requests, request)
	s.mu.Unlock()
	return s.handler(request)
}

func highNouls(req typesafe.Request) (typesafe.Response, error) {
	return noulResponse(req, nil), nil
}

func noulResponse(req typesafe.Request, overrides map[string]float64) typesafe.Response {
	answers := map[string]typesafe.Answer{}
	for id := range req.Questions {
		value := 0.9
		if override, ok := overrides[id]; ok {
			value = override
		}
		copied := value
		answers[id] = typesafe.Answer{Type: "noul", Noul: &copied}
	}
	return typesafe.Response{Model: typesafe.Model, Answers: answers, Usage: typesafe.Usage{InputTokens: 2, OutputTokens: 1}}
}

func typesafeOptions(provider Provider) RecommendOptions {
	return RecommendOptions{
		Tool:              model.ToolCodex,
		Query:             "video helper",
		TaskBrief:         "Render a video clip with a documented workflow.",
		RequestedProvider: ProviderTypeSafe,
		NewProvider:       func(context.Context) (Provider, error) { return provider, nil },
	}
}

func recommendRow(name, description string, tool model.Tool, state model.SkillState) model.SkillRow {
	cell := &model.ToolSkill{Tool: tool, Name: name, Description: description, Group: model.GroupLocal, Source: model.SourceLocal, State: state}
	row := model.SkillRow{Name: name, Description: description, Group: model.GroupLocal, Source: model.SourceLocal}
	switch tool {
	case model.ToolClaude:
		row.Claude = cell
	case model.ToolCodex:
		row.Codex = cell
	case model.ToolMuse:
		row.Muse = cell
	case model.ToolGrok:
		row.Grok = cell
	}
	return row
}

func manyRecommendRows(n int, tool model.Tool) []model.SkillRow {
	rows := make([]model.SkillRow, 0, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("skill-%04d", i)
		rows = append(rows, recommendRow(name, strings.Repeat("video helper ", 40)+name, tool, model.SkillStateOn))
	}
	return rows
}

func candidateNamesContain(rows []model.SkillRow, name string) bool {
	for _, row := range rows {
		if row.Name == name {
			return true
		}
	}
	return false
}

func containsName(names []string, name string) bool {
	for _, item := range names {
		if item == name {
			return true
		}
	}
	return false
}

func requestHasID(req typesafe.Request, id string) bool {
	_, ok := req.Questions[id]
	return ok
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
