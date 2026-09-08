package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/state"
)

type repairCLIOptions struct {
	GitURL string
	DryRun bool
}

func parseRepairArgs(args []string) (repairCLIOptions, error) {
	options := repairCLIOptions{}
	switch len(args) {
	case 1:
		if strings.HasPrefix(args[0], "-") {
			return repairCLIOptions{}, fmt.Errorf("expected repair <git-url> [--dry-run]")
		}
		options.GitURL = strings.TrimSpace(args[0])
	case 2:
		if strings.HasPrefix(args[0], "-") || args[1] != "--dry-run" {
			return repairCLIOptions{}, fmt.Errorf("expected repair <git-url> [--dry-run]")
		}
		options.GitURL = strings.TrimSpace(args[0])
		options.DryRun = true
	default:
		return repairCLIOptions{}, fmt.Errorf("expected repair <git-url> [--dry-run]")
	}
	if options.GitURL == "" {
		return repairCLIOptions{}, fmt.Errorf("repair requires a Git URL")
	}
	return options, nil
}

func (a App) runRepair(stdout, stderr io.Writer, args []string) int {
	options, err := parseRepairArgs(args)
	if err != nil {
		return usageError(stderr, err.Error())
	}
	manifest, err := state.New(a.paths).Load()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	repositories, err := selectRepositories(manifest, options.GitURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	repository := repositories[0]
	group := repositoryGroup(repository)
	service := install.NewRepairService(a.paths, nil)

	if options.DryRun {
		plan, err := service.Plan(repository)
		if err != nil {
			fmt.Fprintf(stderr, "error: repair %s: %v\n", group, err)
			return 1
		}
		fmt.Fprintf(stdout, "dry-run: repair %s\n", group)
		fmt.Fprintf(stdout, "checkout: %s\n", plan.CheckoutPath)
		if plan.Clean {
			fmt.Fprintln(stdout, "checkout is already clean; nothing to repair")
			return 0
		}
		printWorktreeEntries(stdout, plan.Entries)
		fmt.Fprintln(stdout, "would stage these paths under ~/.skill-manager/trash and restore tracked paths from HEAD")
		fmt.Fprintln(stdout, "no files were moved and no Git refs were changed")
		return 0
	}

	result, err := service.Apply(repository)
	if err != nil {
		fmt.Fprintf(stderr, "error: repair %s: %v\n", group, err)
		return 1
	}
	if len(result.StagedEntries) == 0 && len(result.RestoredPaths) == 0 {
		fmt.Fprintf(stdout, "clean %s: nothing to repair\n", group)
		return 0
	}
	fmt.Fprintf(stdout, "repaired %s\n", group)
	printWorktreeEntries(stdout, result.StagedEntries)
	fmt.Fprintf(stdout, "staged %d path(s) through ~/.skill-manager/trash and removed the staging copy\n", len(result.StagedEntries))
	if len(result.RestoredPaths) > 0 {
		fmt.Fprintf(stdout, "restored %d tracked path(s) from HEAD\n", len(result.RestoredPaths))
	}
	if result.CleanupPending != "" {
		fmt.Fprintf(stdout, "staging residue remains at %s\n", result.CleanupPending)
	}
	fmt.Fprintf(stdout, "run \"skill-manager update %s\" to fetch changes\n", repository.OriginalURL)
	return 0
}

func printWorktreeEntries(stdout io.Writer, entries []install.WorktreeEntry) {
	byClass := map[string][]string{}
	for _, entry := range entries {
		byClass[entry.Class] = append(byClass[entry.Class], entry.RelPath)
	}
	for _, class := range []string{install.WorktreeEntryTracked, install.WorktreeEntryUntracked, install.WorktreeEntryIgnored} {
		paths := byClass[class]
		if len(paths) == 0 {
			continue
		}
		sort.Strings(paths)
		fmt.Fprintf(stdout, "%s (%d):\n", class, len(paths))
		for _, path := range paths {
			fmt.Fprintf(stdout, "  %s\n", path)
		}
	}
}

// reportCheckoutConflict adds a classified explanation, the offending paths, and
// the exact next step to a failed repository operation.
func reportCheckoutConflict(stderr io.Writer, repository state.RepositoryEntry, err error) {
	var conflict install.CheckoutConflictError
	if !errors.As(err, &conflict) {
		return
	}
	fmt.Fprintf(stderr, "cause: %s\n", conflict.Cause())
	if conflict.Repairable() {
		printWorktreeEntries(stderr, conflict.Entries)
		fmt.Fprintf(stderr, "run \"skill-manager repair %s\" to stage these paths through ~/.skill-manager/trash, then update again\n", repository.OriginalURL)
		return
	}
	fmt.Fprintf(stderr, "next step: %s\n", conflict.Remedy())
}
