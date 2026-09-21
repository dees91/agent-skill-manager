package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/credentials"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/typesafe"
)

type advisorRecommendOptions struct {
	Tool     model.Tool
	Query    string
	Provider string
	JSON     bool
}

type advisorRecommendOutput struct {
	APIVersion        int             `json:"apiVersion"`
	Tool              model.Tool      `json:"tool"`
	RequestedProvider string          `json:"requestedProvider"`
	UsedProvider      string          `json:"usedProvider"`
	Outcome           string          `json:"outcome"`
	FallbackReason    string          `json:"fallbackReason,omitempty"`
	Candidates        []listJSONSkill `json:"candidates"`
	RecommendedSkills []string        `json:"recommendedSkills"`
	Usage             *advisor.Usage  `json:"usage,omitempty"`
}

func (a App) runAdvisorRecommend(stdout, stderr io.Writer, args []string) int {
	options, err := parseAdvisorRecommendArgs(args)
	if err != nil {
		return advisorUsageError(stderr, containsArgument(args, "--json"), err)
	}
	brief, err := readTaskBrief(a.stdin)
	if err != nil {
		return advisorUsageError(stderr, options.JSON, err)
	}
	requested, settingsUnreadable, err := a.resolveRecommendProvider(options.Provider)
	if err != nil {
		return advisorUsageError(stderr, options.JSON, err)
	}
	rows, err := a.rows()
	if err != nil {
		return advisorCommandError(stderr, options.JSON, "RECOMMEND_FAILED", publicAdvisorSearchError(err, options.JSON), "")
	}
	result, err := advisor.Recommend(context.Background(), rows, advisor.RecommendOptions{
		Tool:               options.Tool,
		Query:              options.Query,
		TaskBrief:          brief,
		RequestedProvider:  requested,
		SettingsUnreadable: settingsUnreadable,
		NewProvider:        a.typesafeProviderFactory,
		Refresh:            a.rows,
	})
	if err != nil {
		return advisorUsageError(stderr, options.JSON, err)
	}
	output := advisorRecommendOutput{
		APIVersion:        result.APIVersion,
		Tool:              result.Tool,
		RequestedProvider: string(result.RequestedProvider),
		UsedProvider:      string(result.UsedProvider),
		Outcome:           result.Outcome,
		FallbackReason:    result.FallbackReason,
		Candidates:        make([]listJSONSkill, 0, len(result.Candidates)),
		RecommendedSkills: result.RecommendedSkills,
		Usage:             result.Usage,
	}
	if output.RecommendedSkills == nil {
		output.RecommendedSkills = []string{}
	}
	for _, row := range result.Candidates {
		output.Candidates = append(output.Candidates, listJSONSkillFromRow(row))
	}
	if options.JSON {
		return writeAdvisorJSON(stdout, stderr, output)
	}
	printAdvisorRecommend(stdout, result)
	return 0
}

func parseAdvisorRecommendArgs(args []string) (advisorRecommendOptions, error) {
	options := advisorRecommendOptions{}
	toolSeen := false
	querySeen := false
	taskStdin := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--tool":
			if toolSeen || index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return advisorRecommendOptions{}, fmt.Errorf("advisor recommend requires one --tool <claude|codex|muse|grok>")
			}
			tool, ok := model.ParseTool(args[index+1])
			if !ok {
				return advisorRecommendOptions{}, fmt.Errorf("invalid tool %q", args[index+1])
			}
			options.Tool = tool
			toolSeen = true
			index++
		case "--query":
			if querySeen || index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return advisorRecommendOptions{}, fmt.Errorf("advisor recommend requires one --query <text>")
			}
			options.Query = args[index+1]
			querySeen = true
			index++
		case "--provider":
			if options.Provider != "" || index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return advisorRecommendOptions{}, fmt.Errorf("advisor recommend accepts one --provider local|typesafe")
			}
			if _, ok := advisor.ParseProviderMode(args[index+1]); !ok {
				return advisorRecommendOptions{}, fmt.Errorf("invalid provider %q", args[index+1])
			}
			options.Provider = args[index+1]
			index++
		case "--task-stdin":
			if taskStdin {
				return advisorRecommendOptions{}, fmt.Errorf("--task-stdin may be provided only once")
			}
			taskStdin = true
		case "--json":
			if options.JSON {
				return advisorRecommendOptions{}, fmt.Errorf("--json may be provided only once")
			}
			options.JSON = true
		default:
			return advisorRecommendOptions{}, fmt.Errorf("unknown advisor recommend argument %q", args[index])
		}
	}
	if !toolSeen {
		return advisorRecommendOptions{}, fmt.Errorf("advisor recommend requires --tool <claude|codex|muse|grok>")
	}
	if !querySeen {
		return advisorRecommendOptions{}, fmt.Errorf("advisor recommend requires --query <text>")
	}
	if !taskStdin {
		return advisorRecommendOptions{}, fmt.Errorf("advisor recommend requires --task-stdin")
	}
	return options, nil
}

func readTaskBrief(stdin io.Reader) (string, error) {
	if stdin == nil {
		return "", fmt.Errorf("task brief is required")
	}
	data, err := io.ReadAll(io.LimitReader(stdin, advisor.MaxTaskBriefBytes+1))
	if err != nil {
		return "", fmt.Errorf("task brief is required")
	}
	if len(data) > advisor.MaxTaskBriefBytes {
		return "", fmt.Errorf("task brief must contain at most %d bytes", advisor.MaxTaskBriefBytes)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("task brief must be valid UTF-8")
	}
	brief := strings.TrimSpace(string(data))
	if brief == "" {
		return "", fmt.Errorf("task brief is required")
	}
	return brief, nil
}

func (a App) resolveRecommendProvider(flag string) (advisor.ProviderMode, bool, error) {
	if flag != "" {
		mode, ok := advisor.ParseProviderMode(flag)
		if !ok {
			return "", false, fmt.Errorf("invalid provider %q", flag)
		}
		return mode, false, nil
	}
	settings, err := advisor.NewSettingsStore(a.paths).Load()
	if err != nil {
		return advisor.ProviderLocal, true, nil
	}
	return settings.Provider, false, nil
}

func (a App) typesafeProviderFactory(ctx context.Context) (advisor.Provider, error) {
	resolved, err := credentials.Resolver{LookupEnv: a.lookupEnv, Store: a.credentials}.Resolve(ctx)
	if err != nil {
		if errors.Is(err, credentials.ErrUnavailable) {
			return nil, &advisor.Fallback{Reason: advisor.FallbackStoreUnavailable}
		}
		if errors.Is(err, credentials.ErrDenied) {
			return nil, &advisor.Fallback{Reason: advisor.FallbackStoreDenied}
		}
		return nil, err
	}
	if resolved.Source == credentials.SourceNone || resolved.Secret == "" {
		return nil, &advisor.Fallback{Reason: advisor.FallbackMissingKey}
	}
	if a.newProvider == nil {
		return nil, &advisor.Fallback{Reason: advisor.FallbackMissingKey}
	}
	return a.newProvider(typesafe.Key(resolved.Secret)), nil
}

func printAdvisorRecommend(stdout io.Writer, result advisor.RecommendResult) {
	fmt.Fprintf(stdout, "Outcome: %s\n", result.Outcome)
	if result.FallbackReason != "" {
		fmt.Fprintf(stdout, "Fallback: %s\n", result.FallbackReason)
	}
	if len(result.RecommendedSkills) == 0 && len(result.Candidates) == 0 {
		fmt.Fprintln(stdout, "No recommendation candidates.")
		return
	}
	recommended := map[string]struct{}{}
	for _, name := range result.RecommendedSkills {
		recommended[name] = struct{}{}
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Rank\tSkill\tState\tRecommended")
	for index, row := range result.Candidates {
		mark := "no"
		if _, ok := recommended[row.Name]; ok {
			mark = "yes"
		}
		fmt.Fprintf(writer, "%d\t%s\t%s\t%s\n", index+1, row.Name, cellState(searchResultCell(row, result.Tool)), mark)
	}
	_ = writer.Flush()
}
