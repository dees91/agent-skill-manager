package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/model"
)

type advisorActivateOptions struct {
	Tool       model.Tool
	SkillNames []string
	DryRun     bool
	JSON       bool
}

type advisorCleanupOptions struct {
	ReceiptID string
	DryRun    bool
	JSON      bool
}

type advisorStatusOptions struct {
	Tool *model.Tool
	JSON bool
}

type advisorSearchOptions struct {
	Tool  model.Tool
	Query string
	Limit int
	JSON  bool
}

type advisorSearchOutput struct {
	APIVersion int             `json:"apiVersion"`
	Tool       model.Tool      `json:"tool"`
	Skills     []listJSONSkill `json:"skills"`
}

type advisorErrorOutput struct {
	APIVersion int                `json:"apiVersion"`
	Error      advisorErrorDetail `json:"error"`
}

type advisorErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	ReceiptID string `json:"receiptId,omitempty"`
}

func (a App) runAdvisor(stdout, stderr io.Writer, args []string) int {
	if len(args) == 0 {
		return usageError(stderr, "expected advisor <search|activate|cleanup|status>")
	}
	switch args[0] {
	case "search":
		options, err := parseAdvisorSearchArgs(args[1:])
		if err != nil {
			return advisorUsageError(stderr, containsArgument(args, "--json"), err)
		}
		searchOptions := advisor.SearchOptions{Tool: options.Tool, Query: options.Query, Limit: options.Limit}
		if err := advisor.ValidateSearchOptions(searchOptions); err != nil {
			return advisorUsageError(stderr, options.JSON, err)
		}
		rows, err := a.rows()
		if err != nil {
			return advisorCommandError(stderr, options.JSON, "SEARCH_FAILED", publicAdvisorSearchError(err, options.JSON), "")
		}
		matches, err := advisor.Search(rows, searchOptions)
		if err != nil {
			return advisorCommandError(stderr, options.JSON, "SEARCH_FAILED", publicAdvisorSearchError(err, options.JSON), "")
		}
		result := advisorSearchOutput{APIVersion: advisor.APIVersion, Tool: options.Tool, Skills: make([]listJSONSkill, 0, len(matches))}
		for _, row := range matches {
			result.Skills = append(result.Skills, listJSONSkillFromRow(row))
		}
		if options.JSON {
			return writeAdvisorJSON(stdout, stderr, result)
		}
		printAdvisorSearch(stdout, options.Tool, matches)
		return 0
	case "activate":
		options, err := parseAdvisorActivateArgs(args[1:])
		if err != nil {
			return advisorUsageError(stderr, containsArgument(args, "--json"), err)
		}
		result, err := advisor.New(a.paths).Activate(options.Tool, options.SkillNames, options.DryRun)
		if err != nil {
			return advisorCommandError(stderr, options.JSON, "ACTIVATION_FAILED", err, result.ReceiptID)
		}
		if options.JSON {
			return writeAdvisorJSON(stdout, stderr, result)
		}
		printAdvisorActivation(stdout, result)
		return 0
	case "cleanup":
		options, err := parseAdvisorCleanupArgs(args[1:])
		if err != nil {
			return advisorUsageError(stderr, containsArgument(args, "--json"), err)
		}
		result, err := advisor.New(a.paths).Cleanup(options.ReceiptID, options.DryRun)
		if err != nil {
			return advisorCommandError(stderr, options.JSON, "CLEANUP_FAILED", err, options.ReceiptID)
		}
		if options.JSON {
			return writeAdvisorJSON(stdout, stderr, result)
		}
		printAdvisorCleanup(stdout, result)
		return 0
	case "status":
		options, err := parseAdvisorStatusArgs(args[1:])
		if err != nil {
			return advisorUsageError(stderr, containsArgument(args, "--json"), err)
		}
		result, err := advisor.New(a.paths).Status(options.Tool)
		if err != nil {
			return advisorCommandError(stderr, options.JSON, "STATUS_FAILED", err, "")
		}
		if options.JSON {
			return writeAdvisorJSON(stdout, stderr, result)
		}
		printAdvisorStatus(stdout, result)
		return 0
	default:
		return advisorUsageError(stderr, containsArgument(args, "--json"), fmt.Errorf("unknown advisor command %q", args[0]))
	}
}

func parseAdvisorSearchArgs(args []string) (advisorSearchOptions, error) {
	options := advisorSearchOptions{Limit: advisor.DefaultSearchLimit}
	toolSeen := false
	querySeen := false
	limitSeen := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--tool":
			if toolSeen || index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return advisorSearchOptions{}, fmt.Errorf("advisor search requires one --tool <claude|codex|muse|grok>")
			}
			tool, ok := model.ParseTool(args[index+1])
			if !ok {
				return advisorSearchOptions{}, fmt.Errorf("invalid tool %q", args[index+1])
			}
			options.Tool = tool
			toolSeen = true
			index++
		case "--query":
			if querySeen || index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return advisorSearchOptions{}, fmt.Errorf("advisor search requires one --query <text>")
			}
			options.Query = args[index+1]
			querySeen = true
			index++
		case "--limit":
			if limitSeen || index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return advisorSearchOptions{}, fmt.Errorf("advisor search accepts one --limit <1-%d>", advisor.MaxSearchLimit)
			}
			limit, err := strconv.Atoi(args[index+1])
			if err != nil {
				return advisorSearchOptions{}, fmt.Errorf("invalid advisor search limit %q", args[index+1])
			}
			options.Limit = limit
			limitSeen = true
			index++
		case "--json":
			if options.JSON {
				return advisorSearchOptions{}, fmt.Errorf("--json may be provided only once")
			}
			options.JSON = true
		default:
			return advisorSearchOptions{}, fmt.Errorf("unknown advisor search argument %q", args[index])
		}
	}
	if !toolSeen {
		return advisorSearchOptions{}, fmt.Errorf("advisor search requires --tool <claude|codex|muse|grok>")
	}
	if !querySeen {
		return advisorSearchOptions{}, fmt.Errorf("advisor search requires --query <text>")
	}
	return options, nil
}

func parseAdvisorActivateArgs(args []string) (advisorActivateOptions, error) {
	options := advisorActivateOptions{}
	toolSeen := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--tool":
			if toolSeen || index+1 >= len(args) {
				return advisorActivateOptions{}, fmt.Errorf("advisor activate requires one --tool <claude|codex|muse|grok>")
			}
			tool, ok := model.ParseTool(args[index+1])
			if !ok {
				return advisorActivateOptions{}, fmt.Errorf("invalid tool %q", args[index+1])
			}
			options.Tool = tool
			toolSeen = true
			index++
		case "--skill":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return advisorActivateOptions{}, fmt.Errorf("--skill requires a value")
			}
			options.SkillNames = append(options.SkillNames, args[index+1])
			index++
		case "--dry-run":
			if options.DryRun {
				return advisorActivateOptions{}, fmt.Errorf("--dry-run may be provided only once")
			}
			options.DryRun = true
		case "--json":
			if options.JSON {
				return advisorActivateOptions{}, fmt.Errorf("--json may be provided only once")
			}
			options.JSON = true
		default:
			return advisorActivateOptions{}, fmt.Errorf("unknown advisor activate argument %q", args[index])
		}
	}
	if !toolSeen {
		return advisorActivateOptions{}, fmt.Errorf("advisor activate requires --tool <claude|codex|muse|grok>")
	}
	if len(options.SkillNames) == 0 {
		return advisorActivateOptions{}, fmt.Errorf("advisor activate requires at least one --skill")
	}
	return options, nil
}

func parseAdvisorCleanupArgs(args []string) (advisorCleanupOptions, error) {
	options := advisorCleanupOptions{}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--receipt":
			if options.ReceiptID != "" || index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return advisorCleanupOptions{}, fmt.Errorf("advisor cleanup requires one --receipt <id>")
			}
			options.ReceiptID = strings.TrimSpace(args[index+1])
			index++
		case "--dry-run":
			if options.DryRun {
				return advisorCleanupOptions{}, fmt.Errorf("--dry-run may be provided only once")
			}
			options.DryRun = true
		case "--json":
			if options.JSON {
				return advisorCleanupOptions{}, fmt.Errorf("--json may be provided only once")
			}
			options.JSON = true
		default:
			return advisorCleanupOptions{}, fmt.Errorf("unknown advisor cleanup argument %q", args[index])
		}
	}
	if options.ReceiptID == "" {
		return advisorCleanupOptions{}, fmt.Errorf("advisor cleanup requires --receipt <id>")
	}
	return options, nil
}

func parseAdvisorStatusArgs(args []string) (advisorStatusOptions, error) {
	options := advisorStatusOptions{}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--tool":
			if options.Tool != nil || index+1 >= len(args) {
				return advisorStatusOptions{}, fmt.Errorf("--tool may be provided only once")
			}
			tool, ok := model.ParseTool(args[index+1])
			if !ok {
				return advisorStatusOptions{}, fmt.Errorf("invalid tool %q", args[index+1])
			}
			options.Tool = &tool
			index++
		case "--json":
			if options.JSON {
				return advisorStatusOptions{}, fmt.Errorf("--json may be provided only once")
			}
			options.JSON = true
		default:
			return advisorStatusOptions{}, fmt.Errorf("unknown advisor status argument %q", args[index])
		}
	}
	return options, nil
}

func printAdvisorActivation(stdout io.Writer, result advisor.ActivateResult) {
	if result.DryRun {
		fmt.Fprintf(stdout, "dry-run: advisor activate %s\n", result.Tool)
	}
	if result.ReceiptID != "" {
		fmt.Fprintf(stdout, "receipt: %s\n", result.ReceiptID)
	}
	for _, action := range result.Actions {
		verb := advisorActionVerb(action.Action, result.DryRun)
		fmt.Fprintf(stdout, "%s %s/%s\n", verb, result.Tool, action.Skill)
	}
	if !result.DryRun && result.ReceiptID == "" {
		fmt.Fprintln(stdout, "no receipt created; all selected skills were already on")
	}
}

func printAdvisorCleanup(stdout io.Writer, result advisor.CleanupResult) {
	if result.DryRun {
		fmt.Fprintf(stdout, "dry-run: advisor cleanup %s\n", result.ReceiptID)
	} else {
		fmt.Fprintf(stdout, "cleaned receipt: %s\n", result.ReceiptID)
	}
	for _, action := range result.Actions {
		verb := advisorActionVerb(action.Action, result.DryRun)
		fmt.Fprintf(stdout, "%s %s/%s\n", verb, result.Tool, action.Skill)
	}
}

func advisorActionVerb(action string, dryRun bool) string {
	label := strings.ReplaceAll(action, "_", " ")
	if !dryRun || action == advisor.ActionAlreadyOn || action == advisor.ActionAlreadyOff {
		return label
	}
	return "would " + label
}

func printAdvisorStatus(stdout io.Writer, result advisor.StatusResult) {
	if len(result.Receipts) == 0 {
		fmt.Fprintln(stdout, "No active advisor receipts.")
		return
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Receipt\tTool\tCreated\tSkills")
	for _, receipt := range result.Receipts {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", receipt.ReceiptID, receipt.Tool, receipt.CreatedAt.Format("2006-01-02T15:04:05Z"), strings.Join(receipt.Skills, ","))
	}
	_ = writer.Flush()
}

func printAdvisorSearch(stdout io.Writer, tool model.Tool, rows []model.SkillRow) {
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No advisor search matches.")
		return
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Rank\tSkill\tState\tGroup\tSource")
	for index, row := range rows {
		fmt.Fprintf(writer, "%d\t%s\t%s\t%s\t%s\n", index+1, row.Name, cellState(searchResultCell(row, tool)), row.Group, row.Source)
	}
	_ = writer.Flush()
}

func searchResultCell(row model.SkillRow, tool model.Tool) *model.ToolSkill {
	switch tool {
	case model.ToolClaude:
		return row.Claude
	case model.ToolCodex:
		return row.Codex
	case model.ToolMuse:
		return row.Muse
	case model.ToolGrok:
		return row.Grok
	default:
		return nil
	}
}

func publicAdvisorSearchError(err error, jsonOutput bool) error {
	if jsonOutput {
		return fmt.Errorf("advisor search could not read the local skill inventory")
	}
	return err
}

func writeAdvisorJSON(stdout, stderr io.Writer, value any) int {
	if err := writeIndentedJSON(stdout, value); err != nil {
		fmt.Fprintf(stderr, "error: encode advisor response: %v\n", err)
		return 1
	}
	return 0
}

func advisorUsageError(stderr io.Writer, jsonOutput bool, err error) int {
	return advisorCommandError(stderr, jsonOutput, "INVALID_ARGUMENT", err, "")
}

func advisorCommandError(stderr io.Writer, jsonOutput bool, code string, err error, receiptID string) int {
	if !jsonOutput {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	output := advisorErrorOutput{APIVersion: advisor.APIVersion, Error: advisorErrorDetail{Code: code, Message: err.Error(), ReceiptID: receiptID}}
	if encodeErr := writeIndentedJSON(stderr, output); encodeErr != nil {
		fmt.Fprintf(stderr, "error: %v; additionally failed to encode advisor error: %v\n", err, encodeErr)
	}
	return 1
}

func containsArgument(args []string, target string) bool {
	for _, argument := range args {
		if argument == target {
			return true
		}
	}
	return false
}
