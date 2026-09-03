package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

type extendCLIOptions struct {
	Tool   model.Tool
	DryRun bool
}

func parseExtendArgs(args []string) (extendCLIOptions, error) {
	options := extendCLIOptions{}
	toolSeen := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--tool":
			if toolSeen || index+1 >= len(args) {
				return extendCLIOptions{}, fmt.Errorf("expected extend --tool <tool> [--dry-run]")
			}
			tool, ok := model.ParseTool(args[index+1])
			if !ok {
				return extendCLIOptions{}, fmt.Errorf("expected extend --tool <tool> [--dry-run]")
			}
			options.Tool = tool
			toolSeen = true
			index++
		case "--dry-run":
			options.DryRun = true
		default:
			return extendCLIOptions{}, fmt.Errorf("expected extend --tool <tool> [--dry-run]")
		}
	}
	if !toolSeen {
		return extendCLIOptions{}, fmt.Errorf("expected extend --tool <tool> [--dry-run]")
	}
	return options, nil
}

func (a App) runExtend(stdout, stderr io.Writer, args []string) int {
	options, err := parseExtendArgs(args)
	if err != nil {
		return usageError(stderr, err.Error())
	}
	manifest, err := state.New(a.paths).Load()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if len(manifest.Repositories) == 0 && len(manifest.LocalSources) == 0 {
		fmt.Fprintln(stdout, "No managed sources recorded.")
		return 0
	}
	if options.DryRun {
		plan, err := install.PlanExtend(a.paths, manifest, options.Tool)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return printExtendDryRun(stdout, stderr, a.paths, options.Tool, plan)
	}
	result, err := install.NewExtendService(a.paths).Apply(options.Tool, nil)
	printExtendCompleted(stdout, result.Completed)
	if err != nil {
		printExtendFailure(stderr, err, result.RolledBack)
		return 1
	}
	created, already, disabled, skipped := extendResultTotals(result)
	ready := 0
	for _, done := range result.Completed {
		if done.Status == install.ExtendStatusExtended {
			ready++
		}
	}
	fmt.Fprintf(stdout, "extended %d symlink(s) across %d source(s); %d already installed; %d disabled; %d skipped\n",
		created, ready, already, disabled, skipped)
	if created > 0 {
		fmt.Fprintln(stdout, "start a new Claude/Codex/Muse/Grok session for guaranteed skill detection")
	}
	return 0
}

func printExtendDryRun(stdout, stderr io.Writer, p paths.Paths, tool model.Tool, plan install.ExtendPlan) int {
	fmt.Fprintf(stdout, "dry-run: extend to %s\n", tool)
	blocked := 0
	for _, source := range plan.Sources {
		label := extendSourceLabel(source.Kind)
		if source.Status == install.ExtendStatusBlocked {
			blocked++
			fmt.Fprintf(stderr, "error: extend --tool %s failed for source %s: %v\n", tool, source.Group, source.Err)
			var planErr install.PlanError
			if errors.As(source.Err, &planErr) {
				printInstallPlanError(stderr, planErr)
			}
			continue
		}
		fmt.Fprintf(stdout, "source %s (%s): would link %d, already installed %d, would disable %d, skipped %d\n",
			source.Group, label, len(source.Links), len(source.AlreadyInstalled), len(source.DisableAfter), len(source.Skipped))
		for _, link := range source.Links {
			fmt.Fprintf(stdout, "would link %s/%s: %s -> %s\n", link.Tool, link.Skill.Name, link.TargetPath, link.Skill.Path)
		}
		for _, already := range source.AlreadyInstalled {
			printAlreadyInstalled(stdout, already)
		}
		for _, name := range source.DisableAfter {
			activeDir, _ := p.UserSkillsDirFor(tool)
			disabledDir, _ := p.DisabledDirFor(tool)
			fmt.Fprintf(stdout, "would disable %s/%s: %s -> %s\n", tool, name, filepath.Join(activeDir, name), filepath.Join(disabledDir, name))
		}
		for _, skipped := range source.Skipped {
			fmt.Fprintf(stdout, "skipped %s: %s\n", skipped.SkillName, skipped.Reason)
		}
	}
	totals := plan.Totals()
	fmt.Fprintf(stdout, "would link %d symlink(s) across %d source(s); %d already installed; %d would be disabled; %d skipped\n",
		totals.Links, totals.Ready, totals.AlreadyInstalled, totals.DisableAfter, totals.SkippedSkills)
	if blocked > 0 {
		return 1
	}
	return 0
}

func printExtendCompleted(stdout io.Writer, completed []install.ExtendSourceResult) {
	for _, done := range completed {
		switch done.Status {
		case install.ExtendStatusExtended:
			fmt.Fprintf(stdout, "extended %s: created %d symlink(s); %d already installed; %d disabled; %d skipped\n",
				done.Group, done.Created, done.AlreadyInstalled, done.Disabled, len(done.Skipped))
		case install.ExtendStatusUnchanged:
			fmt.Fprintf(stdout, "unchanged %s: %s\n", done.Group, done.Reason)
		case install.ExtendStatusSkipped:
			fmt.Fprintf(stdout, "skipped %s: %s\n", done.Group, done.Reason)
		}
	}
}

func printExtendFailure(stderr io.Writer, err error, rolledBack int) {
	var failure *install.ExtendFailure
	if errors.As(err, &failure) {
		fmt.Fprintf(stderr, "error: %v\n", failure)
	} else {
		fmt.Fprintf(stderr, "error: %v\n", err)
	}
	if rolledBack > 0 {
		fmt.Fprintf(stderr, "rolled back %d created symlink(s)\n", rolledBack)
	}
}

func extendResultTotals(result install.ExtendResult) (created, already, disabled, skipped int) {
	for _, done := range result.Completed {
		created += done.Created
		already += done.AlreadyInstalled
		disabled += done.Disabled
		skipped += len(done.Skipped)
	}
	return created, already, disabled, skipped
}

func extendSourceLabel(kind install.ExtendSourceKind) string {
	if kind == install.ExtendSourceLocal {
		return "local path"
	}
	return "git"
}
