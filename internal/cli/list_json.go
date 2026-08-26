package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/model"
)

type listJSONOutput struct {
	APIVersion int             `json:"apiVersion"`
	Skills     []listJSONSkill `json:"skills"`
}

type listJSONSkill struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Group       string        `json:"group"`
	Source      string        `json:"source"`
	Tools       listJSONTools `json:"tools"`
}

type listJSONTools struct {
	Claude listJSONCell `json:"claude"`
	Codex  listJSONCell `json:"codex"`
}

type listJSONCell struct {
	State      string `json:"state"`
	Toggleable bool   `json:"toggleable"`
}

type listJSONOptions struct {
	AvailableFor *model.Tool
	Queries      []string
}

func parseListJSONArgs(args []string) (listJSONOptions, error) {
	options := listJSONOptions{}
	jsonSeen := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			if jsonSeen {
				return listJSONOptions{}, fmt.Errorf("--json may be provided only once")
			}
			jsonSeen = true
		case "--available-for":
			if options.AvailableFor != nil || index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return listJSONOptions{}, fmt.Errorf("list --json requires one --available-for <claude|codex>")
			}
			tool, ok := model.ParseTool(args[index+1])
			if !ok {
				return listJSONOptions{}, fmt.Errorf("invalid tool %q", args[index+1])
			}
			options.AvailableFor = &tool
			index++
		case "--query":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return listJSONOptions{}, fmt.Errorf("--query requires a value")
			}
			query := strings.TrimSpace(args[index+1])
			if query == "" {
				return listJSONOptions{}, fmt.Errorf("--query requires a non-empty value")
			}
			options.Queries = append(options.Queries, strings.ToLower(query))
			index++
		default:
			return listJSONOptions{}, fmt.Errorf("unknown list argument %q", args[index])
		}
	}
	if !jsonSeen {
		return listJSONOptions{}, fmt.Errorf("list filters require --json")
	}
	return options, nil
}

func (a App) runListJSON(stdout, stderr io.Writer, options listJSONOptions) int {
	rows, err := a.rows()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	output := listJSONOutput{APIVersion: advisor.APIVersion, Skills: make([]listJSONSkill, 0, len(rows))}
	for _, row := range rows {
		if !listJSONRowMatches(row, options) {
			continue
		}
		output.Skills = append(output.Skills, listJSONSkillFromRow(row))
	}
	if err := writeIndentedJSON(stdout, output); err != nil {
		fmt.Fprintf(stderr, "error: encode skill list: %v\n", err)
		return 1
	}
	return 0
}

func listJSONSkillFromRow(row model.SkillRow) listJSONSkill {
	return listJSONSkill{
		Name:        row.Name,
		Description: row.Description,
		Group:       row.Group.String(),
		Source:      row.Source.String(),
		Tools: listJSONTools{
			Claude: listCellJSON(row.Claude),
			Codex:  listCellJSON(row.Codex),
		},
	}
}

func listJSONRowMatches(row model.SkillRow, options listJSONOptions) bool {
	if options.AvailableFor != nil {
		var cell *model.ToolSkill
		switch *options.AvailableFor {
		case model.ToolClaude:
			cell = row.Claude
		case model.ToolCodex:
			cell = row.Codex
		}
		if cell == nil || cell.ReadOnly || cell.State != model.SkillStateOff || !cell.Toggleable() {
			return false
		}
	}
	if len(options.Queries) == 0 {
		return true
	}
	metadata := strings.ToLower(strings.Join([]string{row.Name, row.Description, row.Group.String(), row.Source.String()}, " "))
	for _, query := range options.Queries {
		if strings.Contains(metadata, query) {
			return true
		}
	}
	return false
}

func listCellJSON(cell *model.ToolSkill) listJSONCell {
	if cell == nil {
		return listJSONCell{State: "missing", Toggleable: false}
	}
	state := strings.ToLower(cell.State.String())
	if cell.State == model.SkillStateReadOnly {
		state = "read_only"
	}
	toggleable := !cell.ReadOnly && (cell.State == model.SkillStateOn || cell.State == model.SkillStateOff)
	return listJSONCell{State: state, Toggleable: toggleable}
}

func writeIndentedJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
