package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/credentials"
	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/ops"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/scan"
	"github.com/dees91/agent-skill-manager/internal/state"
	"github.com/dees91/agent-skill-manager/internal/tui"
	"github.com/dees91/agent-skill-manager/internal/typesafe"
)

type providerVerifier interface {
	Verify(ctx context.Context) (typesafe.VerifyResult, error)
}

// App contains CLI dependencies.
type App struct {
	paths       paths.Paths
	runTUI      func(paths.Paths) error
	currentDir  func() (string, error)
	stdin       io.Reader
	lookupEnv   func(string) (string, bool)
	credentials credentials.Store
	newProvider func(typesafe.Key) advisor.Provider
	newVerifier func(typesafe.Key) providerVerifier
	readSecret  func(stdin io.Reader, stderr io.Writer, keyStdin bool) (string, error)
}

// Run executes the command-line interface using default user paths.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunWithIO(args, os.Stdin, stdout, stderr)
}

// RunWithIO executes the CLI with an injected stdin, used for task briefs and key input.
func RunWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, err := paths.Default()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	app := newApp(p)
	app.stdin = stdin
	return app.Run(args, stdout, stderr)
}

// RunWithPaths executes the CLI using injected paths, primarily for tests.
func RunWithPaths(args []string, stdout, stderr io.Writer, p paths.Paths) int {
	return newApp(p).Run(args, stdout, stderr)
}

// RunWithPathsAndTUI executes the CLI with an injected TUI runner.
func RunWithPathsAndTUI(args []string, stdout, stderr io.Writer, p paths.Paths, runTUI func(paths.Paths) error) int {
	app := newApp(p)
	app.runTUI = runTUI
	return app.Run(args, stdout, stderr)
}

func newApp(p paths.Paths) App {
	return App{
		paths:       p,
		runTUI:      tui.Run,
		currentDir:  os.Getwd,
		lookupEnv:   os.LookupEnv,
		credentials: credentials.System(),
		newProvider: func(key typesafe.Key) advisor.Provider { return typesafe.New(key) },
		newVerifier: func(key typesafe.Key) providerVerifier { return typesafe.New(key) },
		readSecret:  readHiddenSecret,
	}
}

// Run executes the command-line interface and returns a process exit code.
func (a App) Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return a.runTUICommand(stderr)
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "--version":
		if len(args) != 1 {
			return usageError(stderr, args[0]+" does not accept arguments")
		}
		fmt.Fprintf(stdout, "skill-manager %s\n", currentVersion())
		return 0
	case "tui":
		if len(args) != 1 {
			return usageError(stderr, "tui does not accept arguments")
		}
		return a.runTUICommand(stderr)
	case "list":
		if len(args) == 1 {
			return a.runList(stdout, stderr)
		}
		options, err := parseListJSONArgs(args[1:])
		if err != nil {
			return usageError(stderr, err.Error())
		}
		return a.runListJSON(stdout, stderr, options)
	case "status":
		if len(args) != 1 {
			return usageError(stderr, "status does not accept arguments")
		}
		return a.runStatus(stdout, stderr)
	case "groups":
		if len(args) != 1 {
			return usageError(stderr, "groups does not accept arguments")
		}
		return a.runGroups(stdout, stderr)
	case "repos":
		if len(args) != 1 {
			return usageError(stderr, "repos does not accept arguments")
		}
		return a.runRepos(stdout, stderr)
	case "install":
		return a.runInstall(stdout, stderr, args[1:])
	case "update":
		return a.runUpdate(stdout, stderr, args[1:])
	case "uninstall":
		return a.runUninstall(stdout, stderr, args[1:])
	case "repair":
		return a.runRepair(stdout, stderr, args[1:])
	case "extend":
		return a.runExtend(stdout, stderr, args[1:])
	case "enable":
		return a.runMutation(stdout, stderr, model.OperationEnable, args[1:])
	case "disable":
		return a.runMutation(stdout, stderr, model.OperationDisable, args[1:])
	case "advisor":
		return a.runAdvisor(stdout, stderr, args[1:])
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		fmt.Fprintln(stderr, `Run "skill-manager help" for usage.`)
		return 1
	}
}

func (a App) runUninstall(stdout, stderr io.Writer, args []string) int {
	options, err := parseUninstallArgs(args)
	if err != nil {
		return usageError(stderr, err.Error())
	}
	manifest, err := state.New(a.paths).Load()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	cwd, err := a.currentDir()
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve current directory: %v\n", err)
		return 1
	}
	if install.LooksLikeLocalPathInput(options.GitURL, a.paths.Home, cwd) {
		return a.runLocalUninstall(stdout, stderr, manifest, cwd, options)
	}
	repositories, err := selectRepositories(manifest, options.GitURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	repository := repositories[0]
	service := install.NewUninstallService(a.paths, nil)
	if options.DryRun {
		plan, err := service.Plan(repository)
		if err != nil {
			fmt.Fprintf(stderr, "error: uninstall %s: %v\n", repositoryGroup(repository), err)
			return 1
		}
		fmt.Fprintf(stdout, "dry-run: uninstall %s\n", repositoryGroup(repository))
		for _, reference := range plan.References.References {
			fmt.Fprintf(stdout, "would remove %s %s: %s\n", strings.ToLower(reference.State.String()), cellLabel(reference.Tool, reference.InstalledName, reference.SkillName), reference.LinkPath)
		}
		fmt.Fprintf(stdout, "would remove checkout: %s\n", plan.Checkout.Path)
		fmt.Fprintln(stdout, "would remove repository and matching disabled state entries")
		return 0
	}

	result, err := service.Apply(repository)
	if err != nil {
		fmt.Fprintf(stderr, "error: uninstall %s: %v\n", repositoryGroup(repository), err)
		if len(result.RolledBack) > 0 {
			fmt.Fprintf(stderr, "rolled back %d staged path(s)\n", len(result.RolledBack))
		}
		if result.CleanupPending != "" {
			fmt.Fprintf(stderr, "cleanup pending: %s\n", result.CleanupPending)
		}
		return 1
	}
	fmt.Fprintf(stdout, "removed %d active symlink(s)\n", len(result.RemovedActive))
	fmt.Fprintf(stdout, "removed %d disabled symlink(s)\n", len(result.RemovedDisabled))
	fmt.Fprintf(stdout, "removed checkout: %s\n", result.RemovedCheckout)
	fmt.Fprintf(stdout, "uninstalled %s\n", repositoryGroup(repository))
	fmt.Fprintln(stdout, "start a new Claude/Codex/Muse/Grok session for guaranteed skill detection")
	return 0
}

func (a App) runLocalUninstall(stdout, stderr io.Writer, manifest state.Manifest, cwd string, options uninstallCLIOptions) int {
	lookup, err := install.ResolveLocalSourceLookup(a.paths, cwd, options.GitURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	source, ok := findLocalSource(manifest, lookup)
	if !ok {
		fmt.Fprintf(stderr, "error: local source %s not found in state\n", lookup.OriginalPath)
		return 1
	}
	service := install.NewLocalUninstallService(a.paths)
	if options.DryRun {
		plan, err := service.Plan(source)
		if err != nil {
			fmt.Fprintf(stderr, "error: uninstall %s: %v\n", source.Group, err)
			return 1
		}
		fmt.Fprintf(stdout, "dry-run: uninstall local source %s\n", source.CanonicalPath)
		for _, reference := range plan.References.References {
			fmt.Fprintf(stdout, "would remove %s %s: %s\n", strings.ToLower(reference.State.String()), cellLabel(reference.Tool, reference.InstalledName, reference.SkillName), reference.LinkPath)
		}
		fmt.Fprintln(stdout, "would remove local-source and matching disabled state entries")
		fmt.Fprintf(stdout, "would preserve source: %s\n", source.CanonicalPath)
		return 0
	}
	result, err := service.Apply(source)
	if err != nil {
		fmt.Fprintf(stderr, "error: uninstall %s: %v\n", source.Group, err)
		if len(result.RolledBack) > 0 {
			fmt.Fprintf(stderr, "rolled back %d staged path(s)\n", len(result.RolledBack))
		}
		if result.CleanupPending != "" {
			fmt.Fprintf(stderr, "cleanup pending: %s\n", result.CleanupPending)
		}
		return 1
	}
	fmt.Fprintf(stdout, "removed %d active symlink(s)\n", len(result.RemovedActive))
	fmt.Fprintf(stdout, "removed %d disabled symlink(s)\n", len(result.RemovedDisabled))
	fmt.Fprintf(stdout, "preserved source: %s\n", result.Source.CanonicalPath)
	fmt.Fprintf(stdout, "uninstalled local source %s\n", result.Source.Group)
	fmt.Fprintln(stdout, "start a new Claude/Codex/Muse/Grok session for guaranteed skill detection")
	return 0
}

func (a App) runUpdate(stdout, stderr io.Writer, args []string) int {
	options, err := parseUpdateArgs(args)
	if err != nil {
		return usageError(stderr, err.Error())
	}
	manifest, err := state.New(a.paths).Load()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if options.GitURL != "" {
		cwd, cwdErr := a.currentDir()
		if cwdErr != nil {
			fmt.Fprintf(stderr, "error: resolve current directory: %v\n", cwdErr)
			return 1
		}
		if install.LooksLikeLocalPathInput(options.GitURL, a.paths.Home, cwd) {
			lookup, lookupErr := install.ResolveLocalSourceLookup(a.paths, cwd, options.GitURL)
			if lookupErr != nil {
				fmt.Fprintf(stderr, "error: %v\n", lookupErr)
				return 1
			}
			source, ok := findLocalSource(manifest, lookup)
			if !ok {
				fmt.Fprintf(stderr, "error: local source %s not found in state\n", lookup.OriginalPath)
				return 1
			}
			fmt.Fprintf(stdout, "local source %s is link-in-place and does not require update\n", source.CanonicalPath)
			return 0
		}
	}
	repositories, err := selectRepositories(manifest, options.GitURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if len(repositories) == 0 {
		fmt.Fprintln(stdout, "No managed repositories recorded.")
		return 0
	}

	service := install.NewUpdateService(a.paths, nil)
	if options.DryRun {
		for _, repository := range repositories {
			plan, err := service.PlanLocal(repository)
			if err != nil {
				fmt.Fprintf(stderr, "error: update %s: %v\n", repositoryGroup(repository), err)
				reportCheckoutConflict(stderr, repository, err)
				return 1
			}
			fmt.Fprintf(stdout, "dry-run: update %s\n", repositoryGroup(repository))
			fmt.Fprintf(stdout, "checkout: %s\n", plan.Checkout.Path)
			fmt.Fprintf(stdout, "branch: %s -> %s\n", plan.Checkout.Branch, plan.Checkout.Upstream)
			fmt.Fprintf(stdout, "current commit: %s\n", plan.Checkout.HeadCommit)
			fmt.Fprintln(stdout, "would fetch origin and fast-forward after remote skill-path preflight")
			fmt.Fprintln(stdout, "remote target unavailable without fetch; no Git refs were changed")
			report, discoveryErr := install.NewSkills(plan.Repository)
			discoveryError := ""
			if discoveryErr != nil {
				discoveryError = discoveryErr.Error()
			}
			reportNewSkills(stdout, stderr, plan.Repository, report, discoveryError, a.newSkillSuggester(manifest, plan.Repository, report))
		}
		return 0
	}

	updated := 0
	upToDate := 0
	for _, repository := range repositories {
		result, err := service.Apply(repository)
		if err != nil {
			fmt.Fprintf(stderr, "error: update %s: %v\n", repositoryGroup(repository), err)
			reportCheckoutConflict(stderr, repository, err)
			return 1
		}
		if result.Updated {
			updated++
			fmt.Fprintf(stdout, "updated %s: %s -> %s\n", repositoryGroup(repository), result.PreviousCommit, result.CurrentCommit)
		} else {
			upToDate++
			fmt.Fprintf(stdout, "up-to-date %s: %s\n", repositoryGroup(repository), result.CurrentCommit)
		}
		report := install.NewSkillsReport{Skills: result.NewSkills, Ambiguous: result.AmbiguousNewSkills}
		reportNewSkills(stdout, stderr, result.Repository, report, result.NewSkillsError, a.newSkillSuggester(manifest, result.Repository, report))
	}
	fmt.Fprintf(stdout, "updated %d repository(s); %d up-to-date\n", updated, upToDate)
	if updated > 0 {
		fmt.Fprintln(stdout, "start a new Claude/Codex/Muse/Grok session for guaranteed skill detection")
	}
	return 0
}

func (a App) runInstall(stdout, stderr io.Writer, args []string) int {
	options, err := parseInstallArgs(args)
	if err != nil {
		return usageError(stderr, err.Error())
	}
	cwd, err := a.currentDir()
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve current directory: %v\n", err)
		return 1
	}
	if install.LooksLikeLocalPathInput(options.GitURL, a.paths.Home, cwd) {
		return a.runLocalInstall(stdout, stderr, options, cwd)
	}
	if options.DryRun {
		return a.runInstallDryRun(stdout, stderr, options)
	}
	return a.runInstallApply(stdout, stderr, options)
}

func (a App) runLocalInstall(stdout, stderr io.Writer, options installCLIOptions, cwd string) int {
	source, err := install.ResolveLocalSource(a.paths, cwd, options.GitURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	discovered, err := install.DiscoverLocalSkills(source)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	manifest, err := state.New(a.paths).Load()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	plan, err := install.PlanLocalInstall(a.paths, manifest, source, discovered, install.PlanOptions{Tools: options.Tools, SkillNames: options.SkillNames, Off: options.Off, InstalledAs: options.InstalledAs})
	if err != nil {
		var planErr install.PlanError
		var ambiguous install.AmbiguousSkillsError
		if errors.As(err, &planErr) {
			printInstallPlanError(stderr, planErr)
		} else if errors.As(err, &ambiguous) {
			printAmbiguousSkills(stderr, ambiguous)
		} else {
			fmt.Fprintf(stderr, "error: %v\n", err)
		}
		return 1
	}
	if options.DryRun {
		fmt.Fprintf(stdout, "dry-run: install local source %s\n", source.CanonicalPath)
	} else {
		fmt.Fprintf(stdout, "install local source: %s\n", source.CanonicalPath)
	}
	fmt.Fprintf(stdout, "group: %s\n", source.Group)
	fmt.Fprintf(stdout, "tools: %s\n", formatTools(options.Tools))
	printInstallMode(stdout, options.Off)
	fmt.Fprintf(stdout, "discovered: %d skill(s)\n", discovered.NameCount())
	printResolvedDuplicates(stdout, plan.ResolvedDuplicates)
	if options.DryRun {
		for _, link := range plan.Links {
			printPlannedLink(stdout, link)
		}
		for _, already := range plan.AlreadyInstalled {
			printAlreadyInstalled(stdout, already)
		}
		printOffAlreadyOnNote(stdout, options.Off, plan.AlreadyInstalled)
		fmt.Fprintf(stdout, "would preserve source: %s\n", source.CanonicalPath)
		return 0
	}
	result, err := install.NewLocalApplyService(a.paths).Apply(plan)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		if len(result.RolledBack) > 0 {
			fmt.Fprintf(stderr, "rolled back %d created symlink(s)\n", len(result.RolledBack))
		}
		return 1
	}
	printCreatedLinks(stdout, options.Off, result.Created)
	fmt.Fprintf(stdout, "already installed %d item(s)\n", len(result.AlreadyInstalled))
	for _, already := range result.AlreadyInstalled {
		printAlreadyInstalled(stdout, already)
	}
	printOffAlreadyOnNote(stdout, options.Off, result.AlreadyInstalled)
	printInstalledAliases(stdout, result.Created, result.AlreadyInstalled)
	fmt.Fprintf(stdout, "installed %d skill(s)\n", len(result.Source.InstalledSkills))
	fmt.Fprintf(stdout, "source remains in place: %s\n", result.Source.CanonicalPath)
	fmt.Fprintln(stdout, "start a new Claude/Codex/Muse/Grok session for guaranteed skill detection")
	return 0
}

func (a App) runInstallDryRun(stdout, stderr io.Writer, options installCLIOptions) int {
	prepared, code := a.prepareInstall(stdout, stderr, options, true)
	if code != 0 || prepared.Checkout.WouldClone {
		return code
	}
	for _, link := range prepared.Plan.Links {
		printPlannedLink(stdout, link)
	}
	for _, already := range prepared.Plan.AlreadyInstalled {
		printAlreadyInstalled(stdout, already)
	}
	printOffAlreadyOnNote(stdout, options.Off, prepared.Plan.AlreadyInstalled)
	return 0
}

func (a App) runInstallApply(stdout, stderr io.Writer, options installCLIOptions) int {
	prepared, code := a.prepareInstall(stdout, stderr, options, false)
	if code != 0 {
		return code
	}
	applyResult, err := install.NewApplyService(a.paths).Apply(prepared.Plan, prepared.Checkout.LastSeenCommit)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		if len(applyResult.RolledBack) > 0 {
			fmt.Fprintf(stderr, "rolled back %d created symlink(s)\n", len(applyResult.RolledBack))
		}
		return 1
	}
	if prepared.Checkout.Cloned {
		fmt.Fprintf(stdout, "cloned %s -> %s\n", prepared.Identity.OriginalURL, prepared.CheckoutPath)
	} else if prepared.Checkout.Reused {
		fmt.Fprintf(stdout, "reused checkout %s\n", prepared.CheckoutPath)
	}
	printCreatedLinks(stdout, options.Off, applyResult.Created)
	fmt.Fprintf(stdout, "already installed %d item(s)\n", len(applyResult.AlreadyInstalled))
	for _, already := range applyResult.AlreadyInstalled {
		printAlreadyInstalled(stdout, already)
	}
	printOffAlreadyOnNote(stdout, options.Off, applyResult.AlreadyInstalled)
	printInstalledAliases(stdout, applyResult.Created, applyResult.AlreadyInstalled)
	fmt.Fprintf(stdout, "installed %d skill(s)\n", len(applyResult.Repository.InstalledSkills))
	fmt.Fprintln(stdout, "start a new Claude/Codex/Muse/Grok session for guaranteed skill detection")
	return 0
}

type preparedInstall struct {
	Identity     install.RepoIdentity
	CheckoutPath string
	Checkout     install.CheckoutResult
	Discovered   install.Discovery
	Plan         install.InstallPlan
}

func (a App) prepareInstall(stdout, stderr io.Writer, options installCLIOptions, dryRun bool) (preparedInstall, int) {
	identity, err := install.NormalizeGitURL(options.GitURL)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return preparedInstall{}, 1
	}
	checkoutPath, err := install.CheckoutPath(a.paths, identity)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return preparedInstall{}, 1
	}

	result, err := install.NewCheckoutService(nil).EnsureCheckout(identity, checkoutPath, install.CheckoutOptions{DryRun: dryRun})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return preparedInstall{}, 1
	}

	if dryRun {
		fmt.Fprintf(stdout, "dry-run: install %s\n", identity.OriginalURL)
	} else {
		fmt.Fprintf(stdout, "install: %s\n", identity.OriginalURL)
	}
	fmt.Fprintf(stdout, "checkout: %s\n", checkoutPath)
	fmt.Fprintf(stdout, "tools: %s\n", formatTools(options.Tools))
	printInstallMode(stdout, options.Off)
	if result.WouldClone {
		fmt.Fprintf(stdout, "would clone %s -> %s\n", identity.OriginalURL, checkoutPath)
		fmt.Fprintln(stdout, "skill discovery unavailable until real install")
		return preparedInstall{Identity: identity, CheckoutPath: checkoutPath, Checkout: result}, 0
	}

	discovered, err := install.DiscoverSkills(checkoutPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return preparedInstall{}, 1
	}
	fmt.Fprintf(stdout, "discovered: %d skill(s)\n", discovered.NameCount())
	manifest, err := state.New(a.paths).Load()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return preparedInstall{}, 1
	}
	plan, err := install.PlanInstall(a.paths, manifest, identity, checkoutPath, discovered, install.PlanOptions{
		Tools:       options.Tools,
		SkillNames:  options.SkillNames,
		Off:         options.Off,
		InstalledAs: options.InstalledAs,
	})
	if err != nil {
		var planErr install.PlanError
		var ambiguous install.AmbiguousSkillsError
		if errors.As(err, &planErr) {
			printInstallPlanError(stderr, planErr)
		} else if errors.As(err, &ambiguous) {
			printAmbiguousSkills(stderr, ambiguous)
		} else {
			fmt.Fprintf(stderr, "error: %v\n", err)
		}
		return preparedInstall{}, 1
	}
	printResolvedDuplicates(stdout, plan.ResolvedDuplicates)
	return preparedInstall{
		Identity:     identity,
		CheckoutPath: checkoutPath,
		Checkout:     result,
		Discovered:   discovered,
		Plan:         plan,
	}, 0
}

func (a App) runList(stdout, stderr io.Writer) int {
	rows, err := a.rows()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Skill\tClaude\tCodex\tMuse\tGrok\tSource")
	for _, row := range rows {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n", row.Name, cellState(row.Claude), cellState(row.Codex), cellState(row.Muse), cellState(row.Grok), row.Source)
	}
	_ = writer.Flush()
	return 0
}

func (a App) runStatus(stdout, stderr io.Writer) int {
	rows, err := a.rows()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	counts := statusCounts{}
	for _, row := range rows {
		countCell(row.Claude, &counts)
		countCell(row.Codex, &counts)
		countCell(row.Muse, &counts)
		countCell(row.Grok, &counts)
	}

	fmt.Fprintf(stdout, "ON: %d\n", counts.on)
	fmt.Fprintf(stdout, "OFF: %d\n", counts.off)
	fmt.Fprintf(stdout, "CONFLICT: %d\n", counts.conflict)
	fmt.Fprintf(stdout, "RO: %d\n", counts.readOnly)
	return 0
}

func (a App) runGroups(stdout, stderr io.Writer) int {
	rows, err := a.rows()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Group\tRows\tClaude\tCodex\tMuse\tGrok\tSources")
	for _, summary := range scan.GroupSummaries(rows) {
		fmt.Fprintf(
			writer,
			"%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			summary.Group.String(),
			summary.Rows,
			formatToolCounts(summary.Claude),
			formatToolCounts(summary.Codex),
			formatToolCounts(summary.Muse),
			formatToolCounts(summary.Grok),
			groupSources(summary),
		)
	}
	_ = writer.Flush()
	return 0
}

func (a App) runRepos(stdout, stderr io.Writer) int {
	manifest, err := state.New(a.paths).Load()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if len(manifest.Repositories) == 0 {
		fmt.Fprintln(stdout, "No managed repositories recorded.")
		return 0
	}

	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Group\tURL\tCheckout\tCommit\tSkills\tTools")
	for _, repo := range manifest.Repositories {
		fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\t%d\t%s\n",
			repositoryGroup(repo),
			repositoryURL(repo),
			valueOrDash(repo.CheckoutPath),
			valueOrDash(repo.LastSeenCommit),
			len(repo.InstalledSkills),
			repositoryTools(repo),
		)
	}
	_ = writer.Flush()
	return 0
}

func (a App) rows() ([]model.SkillRow, error) {
	skills, err := scan.New(a.paths).All()
	if err != nil {
		return nil, err
	}
	return scan.RowsFromSkillsWithOptions(skills, scan.RowOptions{IncludeReadOnly: true}), nil
}

func (a App) runMutation(stdout, stderr io.Writer, kind model.OperationKind, args []string) int {
	tool, skillName, dryRun, err := parseMutationArgs(args)
	if err != nil {
		return usageError(stderr, err.Error())
	}

	if err := a.rejectReadOnly(tool, skillName); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	service := ops.New(a.paths)
	var operation model.PlannedOperation
	switch kind {
	case model.OperationDisable:
		operation, err = service.PlanDisable(tool, skillName)
	case model.OperationEnable:
		operation, err = service.PlanEnable(tool, skillName)
	default:
		err = fmt.Errorf("unsupported operation %q", kind)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if dryRun {
		fmt.Fprintf(stdout, "dry-run: %s %s/%s: %s -> %s\n", kind, tool, skillName, operation.FromPath, operation.ToPath)
		return 0
	}

	result := service.Apply([]model.PlannedOperation{operation})
	if result.Failed != nil {
		fmt.Fprintf(stderr, "error: %v\n", result.Failed.Err)
		return 1
	}

	switch kind {
	case model.OperationDisable:
		fmt.Fprintf(stdout, "disabled %s/%s\n", tool, skillName)
	case model.OperationEnable:
		fmt.Fprintf(stdout, "enabled %s/%s\n", tool, skillName)
	}
	return 0
}

func (a App) rejectReadOnly(tool model.Tool, skillName string) error {
	skills, err := scan.New(a.paths).All()
	if err != nil {
		return err
	}
	hasReadOnly := false
	hasMutable := false
	for _, skill := range skills {
		if skill.Tool != tool || skill.Name != skillName {
			continue
		}
		if skill.ReadOnly || skill.State == model.SkillStateReadOnly {
			hasReadOnly = true
			continue
		}
		hasMutable = true
	}
	if hasReadOnly && !hasMutable {
		return fmt.Errorf("%s/%s is read-only", tool, skillName)
	}
	return nil
}

func parseMutationArgs(args []string) (model.Tool, string, bool, error) {
	if len(args) != 3 && len(args) != 4 {
		return "", "", false, fmt.Errorf("expected --tool <claude|codex|muse|grok> <skill> [--dry-run]")
	}
	if args[0] != "--tool" {
		return "", "", false, fmt.Errorf("expected --tool <claude|codex|muse|grok> <skill> [--dry-run]")
	}
	dryRun := false
	if len(args) == 4 {
		if args[3] != "--dry-run" {
			return "", "", false, fmt.Errorf("expected --tool <claude|codex|muse|grok> <skill> [--dry-run]")
		}
		dryRun = true
	}
	tool, ok := model.ParseTool(args[1])
	if !ok {
		return "", "", false, fmt.Errorf("invalid tool %q", args[1])
	}
	skillName := strings.TrimSpace(args[2])
	if skillName == "" {
		return "", "", false, fmt.Errorf("skill name is required")
	}
	return tool, skillName, dryRun, nil
}

type installCLIOptions struct {
	GitURL      string
	Tools       []model.Tool
	SkillNames  []string
	InstalledAs map[string]string
	DryRun      bool
	Off         bool
}

type updateCLIOptions struct {
	GitURL string
	DryRun bool
}

type uninstallCLIOptions struct {
	GitURL string
	DryRun bool
}

func parseUninstallArgs(args []string) (uninstallCLIOptions, error) {
	if len(args) != 1 && len(args) != 2 {
		return uninstallCLIOptions{}, fmt.Errorf("expected uninstall <git-url|local-path> [--dry-run]")
	}
	if strings.HasPrefix(args[0], "-") || strings.TrimSpace(args[0]) == "" {
		return uninstallCLIOptions{}, fmt.Errorf("expected uninstall <git-url|local-path> [--dry-run]")
	}
	options := uninstallCLIOptions{GitURL: strings.TrimSpace(args[0])}
	if len(args) == 2 {
		if args[1] != "--dry-run" {
			return uninstallCLIOptions{}, fmt.Errorf("expected uninstall <git-url|local-path> [--dry-run]")
		}
		options.DryRun = true
	}
	return options, nil
}

func parseUpdateArgs(args []string) (updateCLIOptions, error) {
	options := updateCLIOptions{}
	switch len(args) {
	case 0:
		return options, nil
	case 1:
		if args[0] == "--dry-run" {
			options.DryRun = true
			return options, nil
		}
		if strings.HasPrefix(args[0], "-") {
			return updateCLIOptions{}, fmt.Errorf("unknown update flag %q", args[0])
		}
		options.GitURL = strings.TrimSpace(args[0])
	case 2:
		if strings.HasPrefix(args[0], "-") || args[1] != "--dry-run" {
			return updateCLIOptions{}, fmt.Errorf("expected update [<git-url|local-path>] [--dry-run]")
		}
		options.GitURL = strings.TrimSpace(args[0])
		options.DryRun = true
	default:
		return updateCLIOptions{}, fmt.Errorf("expected update [<git-url|local-path>] [--dry-run]")
	}
	if options.GitURL == "" {
		return updateCLIOptions{}, fmt.Errorf("update source is required when a target is provided")
	}
	return options, nil
}

func selectRepositories(manifest state.Manifest, gitURL string) ([]state.RepositoryEntry, error) {
	if strings.TrimSpace(gitURL) == "" {
		return append([]state.RepositoryEntry(nil), manifest.Repositories...), nil
	}
	identity, err := install.NormalizeGitURL(gitURL)
	if err != nil {
		return nil, err
	}
	repository, ok := manifest.GetRepository(identity.Host, identity.RepoPath)
	if !ok {
		return nil, fmt.Errorf("managed repository %s/%s not found in state", identity.Host, identity.RepoPath)
	}
	return []state.RepositoryEntry{repository}, nil
}

func findLocalSource(manifest state.Manifest, lookup install.LocalSourceLookup) (state.LocalSourceEntry, bool) {
	if source, ok := manifest.GetLocalSource(lookup.CanonicalPath); ok {
		return source, true
	}
	for _, source := range manifest.LocalSources {
		if filepath.Clean(source.OriginalPath) == filepath.Clean(lookup.OriginalPath) {
			return source, true
		}
	}
	return state.LocalSourceEntry{}, false
}

func parseInstallArgs(args []string) (installCLIOptions, error) {
	if len(args) == 0 {
		return installCLIOptions{}, fmt.Errorf("expected install <git-url|local-path> [--tool claude|codex|muse|grok|both|all] [--skill name[=path]...] [--as name=installed-name...] [--off] [--dry-run]")
	}
	if strings.HasPrefix(args[0], "-") {
		return installCLIOptions{}, fmt.Errorf("expected install <git-url|local-path> [--tool claude|codex|muse|grok|both|all] [--skill name[=path]...] [--as name=installed-name...] [--off] [--dry-run]")
	}
	options := installCLIOptions{GitURL: strings.TrimSpace(args[0])}
	if options.GitURL == "" {
		return installCLIOptions{}, fmt.Errorf("install source is required")
	}
	tools, err := install.ParseToolTarget("")
	if err != nil {
		return installCLIOptions{}, err
	}
	options.Tools = tools

	toolSeen := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			options.DryRun = true
		case "--off":
			options.Off = true
		case "--tool":
			if toolSeen {
				return installCLIOptions{}, fmt.Errorf("--tool may be provided only once")
			}
			if i+1 >= len(args) {
				return installCLIOptions{}, fmt.Errorf("--tool requires a value")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return installCLIOptions{}, fmt.Errorf("--tool requires a value")
			}
			tools, err := install.ParseToolTarget(args[i+1])
			if err != nil {
				return installCLIOptions{}, err
			}
			options.Tools = tools
			toolSeen = true
			i++
		case "--skill":
			if i+1 >= len(args) {
				return installCLIOptions{}, fmt.Errorf("--skill requires a value")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return installCLIOptions{}, fmt.Errorf("--skill requires a value")
			}
			name := strings.TrimSpace(args[i+1])
			if name == "" {
				return installCLIOptions{}, fmt.Errorf("--skill requires a non-empty value")
			}
			options.SkillNames = append(options.SkillNames, name)
			i++
		case "--as":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return installCLIOptions{}, fmt.Errorf("--as requires a <name>=<installed-name> value")
			}
			name, installed, err := parseInstalledAs(args[i+1])
			if err != nil {
				return installCLIOptions{}, err
			}
			if options.InstalledAs == nil {
				options.InstalledAs = map[string]string{}
			}
			if previous, ok := options.InstalledAs[name]; ok && previous != installed {
				return installCLIOptions{}, fmt.Errorf("--as gives skill %q two install names: %q and %q", name, previous, installed)
			}
			options.InstalledAs[name] = installed
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return installCLIOptions{}, fmt.Errorf("unknown install flag %q", args[i])
			}
			return installCLIOptions{}, fmt.Errorf("unexpected install argument %q", args[i])
		}
	}
	return options, nil
}

// parseInstalledAs splits one --as value on its first "=" into a source
// skill name and the install name used for its links.
func parseInstalledAs(raw string) (string, string, error) {
	name, installed, ok := strings.Cut(raw, "=")
	name, installed = strings.TrimSpace(name), strings.TrimSpace(installed)
	if !ok || name == "" || installed == "" {
		return "", "", fmt.Errorf("--as requires a <name>=<installed-name> value, got %q", printableText(raw))
	}
	if !state.ValidInstalledName(installed) {
		return "", "", fmt.Errorf("invalid install name %q: use 1-64 lowercase letters, digits, and single hyphens", printableText(installed))
	}
	return name, installed, nil
}

type statusCounts struct {
	on       int
	off      int
	conflict int
	readOnly int
}

func countCell(skill *model.ToolSkill, counts *statusCounts) {
	if skill == nil {
		return
	}
	switch skill.State {
	case model.SkillStateOn:
		counts.on++
	case model.SkillStateOff:
		counts.off++
	case model.SkillStateConflict:
		counts.conflict++
	case model.SkillStateReadOnly:
		counts.readOnly++
	}
}

func formatToolCounts(counts model.ToolStateCounts) string {
	return fmt.Sprintf("ON:%d OFF:%d CONFLICT:%d RO:%d", counts.On, counts.Off, counts.Conflict, counts.ReadOnly)
}

func groupSources(summary model.GroupSummary) string {
	if summary.SourceText == "" {
		return model.SourceUnknown.String()
	}
	return summary.SourceText
}

// reportNewSkills lists skills present in a managed checkout but not recorded
// as installed, with the install commands that add them for the tools the
// repository already uses. Update itself never installs them. Ambiguous names
// list every copy with a qualified --skill template instead of a command.
//
// When another source owns a new skill's plain name, suggest returns a free
// install name and the pasteable command carries the matching --as.
func reportNewSkills(stdout, stderr io.Writer, repository state.RepositoryEntry, report install.NewSkillsReport, discoveryError string, suggest func(string) string) {
	if discoveryError != "" {
		fmt.Fprintf(stderr, "warning: could not check for new skills in %s: %s\n", repositoryGroup(repository), printableText(discoveryError))
		return
	}
	if len(report.Skills) > 0 {
		// Skill names come from upstream directory names, so the pasteable command
		// quotes every argument and is omitted when a name cannot be shown safely.
		names := make([]string, len(report.Skills))
		printable := true
		for i, skill := range report.Skills {
			names[i] = printableText(skill.Name)
			printable = printable && names[i] == skill.Name
		}
		fmt.Fprintf(stdout, "new skills in %s (not installed): %s\n", repositoryGroup(repository), strings.Join(names, ", "))
		if !printable {
			fmt.Fprintln(stdout, "  install command omitted: a skill name contains control characters")
		} else {
			args := []string{"skill-manager", "install", shellQuote(repositoryURL(repository))}
			for _, skill := range report.Skills {
				args = append(args, "--skill", shellQuote(skill.Name))
			}
			for _, skill := range report.Skills {
				if suggest == nil {
					break
				}
				if installed := suggest(skill.Name); installed != "" {
					fmt.Fprintf(stdout, "  %s is owned by another source; the command below installs it as %s\n", skill.Name, installed)
					args = append(args, "--as", shellQuote(skill.Name+"="+installed))
				}
			}
			printNewSkillsCommands(stdout, strings.Join(args, " "), recordedRepositoryTools(repository))
		}
	}
	for _, group := range report.Ambiguous {
		name := printableText(group.Name)
		paths := make([]string, len(group.Paths))
		printable := name == group.Name
		for i, path := range group.Paths {
			paths[i] = printableText(path)
			printable = printable && paths[i] == path
		}
		fmt.Fprintf(stdout, "ambiguous new skill in %s (not installed): %s\n", repositoryGroup(repository), name)
		fmt.Fprintf(stdout, "  %s has %d copies with differing content: %s\n", name, len(paths), strings.Join(paths, ", "))
		if !printable {
			fmt.Fprintln(stdout, "  install command omitted: a skill name or path contains control characters")
			continue
		}
		template := strings.Join([]string{"skill-manager", "install", shellQuote(repositoryURL(repository)), "--skill", shellQuote(group.Name + "=<path>")}, " ")
		printNewSkillsCommands(stdout, template, recordedRepositoryTools(repository))
	}
}

// newSkillSuggester suggests install names for new skills whose plain name is
// owned by another recorded source for one of the repository's tools.
func (a App) newSkillSuggester(manifest state.Manifest, repository state.RepositoryEntry, report install.NewSkillsReport) func(string) string {
	tools := recordedRepositoryTools(repository)
	if len(tools) == 0 {
		tools = model.Tools()
	}
	names := []string{}
	for _, skill := range repository.InstalledSkills {
		names = append(names, skill.Name)
	}
	for _, skill := range report.Skills {
		names = append(names, skill.Name)
	}
	for _, group := range report.Ambiguous {
		names = append(names, group.Name)
	}
	return func(name string) string {
		return install.NewSkillInstalledNameSuggestion(a.paths, manifest, repository, name, tools, names)
	}
}

func printNewSkillsCommands(stdout io.Writer, command string, tools []model.Tool) {
	if len(tools) == 0 || len(tools) == len(model.Tools()) {
		fmt.Fprintf(stdout, "  install: %s\n", command)
		return
	}
	for _, tool := range tools {
		fmt.Fprintf(stdout, "  install: %s --tool %s\n", command, tool)
	}
}

// shellQuote returns value as one POSIX shell word.
func shellQuote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n'\"\\$`;&|<>()*?[]#~{}!=%^") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// printableText returns value unchanged when every rune is printable and a Go
// quoted form otherwise, so terminal control sequences are never emitted.
func printableText(value string) string {
	for _, r := range value {
		if !unicode.IsPrint(r) {
			return strconv.Quote(value)
		}
	}
	return value
}

// recordedRepositoryTools returns the known recorded tools in canonical order.
func recordedRepositoryTools(repository state.RepositoryEntry) []model.Tool {
	recorded := map[model.Tool]bool{}
	for _, skill := range repository.InstalledSkills {
		for _, tool := range skill.Tools {
			recorded[tool] = true
		}
	}
	tools := []model.Tool{}
	for _, tool := range model.Tools() {
		if recorded[tool] {
			tools = append(tools, tool)
		}
	}
	return tools
}

func repositoryGroup(repo state.RepositoryEntry) string {
	if strings.TrimSpace(repo.Group.String()) != "" {
		return repo.Group.String()
	}
	return repositoryIdentity(repo)
}

func repositoryURL(repo state.RepositoryEntry) string {
	if strings.TrimSpace(repo.OriginalURL) != "" {
		return repo.OriginalURL
	}
	if strings.TrimSpace(repo.CanonicalURL) != "" {
		return repo.CanonicalURL
	}
	return repositoryIdentity(repo)
}

func repositoryIdentity(repo state.RepositoryEntry) string {
	switch {
	case strings.TrimSpace(repo.Host) != "" && strings.TrimSpace(repo.RepoPath) != "":
		return strings.TrimSpace(repo.Host) + "/" + strings.TrimSpace(repo.RepoPath)
	case strings.TrimSpace(repo.Host) != "":
		return strings.TrimSpace(repo.Host)
	case strings.TrimSpace(repo.RepoPath) != "":
		return strings.TrimSpace(repo.RepoPath)
	default:
		return "-"
	}
}

func repositoryTools(repo state.RepositoryEntry) string {
	seen := map[model.Tool]bool{}
	for _, skill := range repo.InstalledSkills {
		for _, tool := range skill.Tools {
			seen[tool] = true
		}
	}
	if len(seen) == 0 {
		return "-"
	}

	ordered := []string{}
	for _, tool := range model.Tools() {
		if seen[tool] {
			ordered = append(ordered, tool.String())
			delete(seen, tool)
		}
	}
	unknown := make([]string, 0, len(seen))
	for tool := range seen {
		unknown = append(unknown, tool.String())
	}
	sort.Strings(unknown)
	ordered = append(ordered, unknown...)
	return strings.Join(ordered, ",")
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func formatTools(tools []model.Tool) string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.String()
	}
	return strings.Join(names, ",")
}

func printInstallPlanError(stderr io.Writer, err install.PlanError) {
	for _, missing := range err.MissingSkills {
		fmt.Fprintf(stderr, "missing skill: %s\n", missing)
	}
	defer printInstalledAsSuggestions(stderr, err.Conflicts)
	for _, conflict := range err.Conflicts {
		fmt.Fprintf(stderr, "conflict %s/%s: %s", conflict.Tool, conflict.SkillName, conflict.Reason)
		if conflict.TargetPath != "" {
			fmt.Fprintf(stderr, " at %s", conflict.TargetPath)
		}
		if conflict.Expected != "" {
			fmt.Fprintf(stderr, " expected %s", conflict.Expected)
		}
		if conflict.Existing != "" {
			fmt.Fprintf(stderr, " existing %s", conflict.Existing)
		}
		if conflict.Disabled != "" {
			fmt.Fprintf(stderr, " disabled %s", conflict.Disabled)
		}
		if conflict.Description != "" {
			fmt.Fprintf(stderr, " (%s)", conflict.Description)
		}
		fmt.Fprintln(stderr)
	}
}

// printInstalledAsSuggestions prints, once per skill, the pasteable --as form
// that installs a skill whose plain name another source owns. Values with
// control characters show a quoted form without the pasteable line.
func printInstalledAsSuggestions(stderr io.Writer, conflicts []install.PreflightConflict) {
	type suggestion struct {
		owner string
		as    string
		tools []string
	}
	order := []string{}
	byName := map[string]*suggestion{}
	for _, conflict := range conflicts {
		if conflict.SuggestedAs == "" {
			continue
		}
		item, ok := byName[conflict.SkillName]
		if !ok {
			item = &suggestion{owner: strings.TrimPrefix(conflict.Reason, "cell is already owned by "), as: conflict.SuggestedAs}
			byName[conflict.SkillName] = item
			order = append(order, conflict.SkillName)
		}
		item.tools = append(item.tools, conflict.Tool.String())
	}
	for _, name := range order {
		item := byName[name]
		fmt.Fprintf(stderr, "skill %q is owned by %s for %s; install it under another name:\n", name, printableText(item.owner), strings.Join(item.tools, ","))
		value := name + "=" + item.as
		if printableText(value) != value {
			fmt.Fprintf(stderr, "  %s (pasteable --as form omitted: contains control characters)\n", printableText(value))
			continue
		}
		fmt.Fprintf(stderr, "  --as %s\n", shellQuote(value))
	}
}

// cellLabel names a tool cell by its installed name, adding the source skill
// name when the skill is installed under another name.
func cellLabel(tool model.Tool, installedName, skillName string) string {
	if installedName == "" || installedName == skillName {
		return tool.String() + "/" + skillName
	}
	return tool.String() + "/" + installedName + " (" + skillName + ")"
}

// printInstalledAliases confirms every skill installed under another name.
func printInstalledAliases(stdout io.Writer, created []install.LinkPlan, already []install.AlreadyInstalled) {
	seen := map[string]bool{}
	report := func(name, installed string) {
		if installed == "" || seen[name] {
			return
		}
		seen[name] = true
		fmt.Fprintf(stdout, "installed %q as %q\n", name, installed)
	}
	for _, link := range created {
		report(link.Skill.Name, link.InstalledAs)
	}
	for _, item := range already {
		report(item.Skill.Name, item.InstalledAs)
	}
}

func printInstallMode(stdout io.Writer, off bool) {
	if off {
		fmt.Fprintln(stdout, "mode: install as OFF")
	}
}

// printResolvedDuplicates names the canonical copy used for every identical
// group in the plan, so an automatic choice is never silent. Names and paths
// come from upstream directories and are sanitized before printing.
func printResolvedDuplicates(stdout io.Writer, resolved []install.ResolvedDuplicate) {
	for _, item := range resolved {
		fmt.Fprintf(stdout, "resolved %q: %d identical copies, using %s\n", item.Name, item.Copies, printableText(item.RelativePath))
	}
}

// printAmbiguousSkills lists every copy of each conflicting skill name with
// the qualified --skill form that installs it. Each form is shell-quoted so
// it pastes as one argument; names or paths with control characters show a
// quoted form with the pasteable line omitted instead.
func printAmbiguousSkills(stderr io.Writer, err install.AmbiguousSkillsError) {
	for _, missing := range err.Missing {
		fmt.Fprintf(stderr, "missing skill: %s\n", printableText(missing))
	}
	for _, group := range err.Groups {
		name := printableText(group.Name)
		fmt.Fprintf(stderr, "ambiguous skill %q: %d copies with differing content; qualify with --skill %s=<path>:\n", group.Name, len(group.Paths), name)
		for i, path := range group.Paths {
			if name != group.Name || printableText(path) != path {
				fmt.Fprintf(stderr, "  %s (pasteable --skill form omitted: contains control characters)\n", printableText(group.Name+"="+path))
				continue
			}
			hash := ""
			if i < len(group.Hashes) && group.Hashes[i] != "" && group.Hashes[i] != "-" {
				hash = "  # " + group.Hashes[i]
			}
			fmt.Fprintf(stderr, "  --skill %s%s\n", shellQuote(group.Name+"="+path), hash)
		}
	}
}

func printPlannedLink(stdout io.Writer, link install.LinkPlan) {
	if link.DisabledPath != "" {
		fmt.Fprintf(stdout, "would link %s OFF: %s -> %s (restores to %s)\n", cellLabel(link.Tool, link.InstalledName(), link.Skill.Name), link.DisabledPath, link.Skill.Path, link.TargetPath)
		return
	}
	fmt.Fprintf(stdout, "would link %s: %s -> %s\n", cellLabel(link.Tool, link.InstalledName(), link.Skill.Name), link.TargetPath, link.Skill.Path)
}

func printCreatedLinks(stdout io.Writer, off bool, created []install.LinkPlan) {
	if !off {
		fmt.Fprintf(stdout, "created %d symlink(s)\n", len(created))
		return
	}
	fmt.Fprintf(stdout, "created %d symlink(s) as OFF\n", len(created))
	if len(created) > 0 {
		fmt.Fprintf(stdout, "turn a skill ON with: skill-manager enable --tool %s %s\n", created[0].Tool, shellQuote(created[0].InstalledName()))
	}
}

func printOffAlreadyOnNote(stdout io.Writer, off bool, already []install.AlreadyInstalled) {
	if !off {
		return
	}
	for _, item := range already {
		if item.State == model.SkillStateOn {
			fmt.Fprintln(stdout, "note: --off does not disable already installed skills")
			return
		}
	}
}

func printAlreadyInstalled(stdout io.Writer, already install.AlreadyInstalled) {
	switch already.State {
	case model.SkillStateOn:
		fmt.Fprintf(stdout, "already installed %s: ON at %s\n", cellLabel(already.Tool, already.InstalledName(), already.Skill.Name), already.TargetPath)
	case model.SkillStateOff:
		fmt.Fprintf(stdout, "already installed %s: OFF at %s\n", cellLabel(already.Tool, already.InstalledName(), already.Skill.Name), already.DisabledPath)
	}
}

func cellState(skill *model.ToolSkill) string {
	if skill == nil {
		return model.SkillStateMissing.String()
	}
	return skill.State.String()
}

func usageError(stderr io.Writer, message string) int {
	fmt.Fprintf(stderr, "error: %s\n", message)
	fmt.Fprintln(stderr, `Run "skill-manager help" for usage.`)
	return 1
}

func (a App) runTUICommand(stderr io.Writer) int {
	if err := a.runTUI(a.paths); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func printUsage(stdout io.Writer) {
	fmt.Fprint(stdout, `Usage:
  skill-manager [command]

Commands:
  tui                          Open the terminal UI
  version                      Print the Skill Manager version
  list [--json [--available-for <tool>] [--query <text>...]]
                               List skills or query path-free JSON metadata
  status                       Summarize skill states
  groups                       Summarize detected groups
  repos                        Summarize managed repository installs
  install <git-url|local-path> [--tool <tool>] [--skill <name>...] [--as <name>=<installed-name>...] [--off] [--dry-run]
                               Install or preview skills from Git or a local path;
                               --off creates them disabled; --as links a skill
                               under another name when its name is taken
  update [<git-url|local-path>] [--dry-run]
                               Update one or all managed repositories
  uninstall <git-url|local-path> [--dry-run]
                               Remove a managed source and all its links
  repair <git-url> [--dry-run]
                               Clear worktree changes blocking a managed checkout
  extend --tool <tool> [--dry-run]
                               Link recorded skills to one more tool across every managed source
  enable --tool <tool> <skill> [--dry-run]
                               Restore a disabled skill
  disable --tool <tool> <skill> [--dry-run]
                               Disable an active skill
  advisor activate --tool <tool> --skill <name>... [--dry-run] [--json]
                               Activate a receipt-scoped skill set
  advisor search --tool <tool> --query <text> [--limit <n>] [--json]
                               Rank tool-specific ON/OFF skill metadata
  advisor recommend --tool <tool> --query <text> --task-stdin [--provider local|typesafe] [--json]
                               Recommend skills from local search or TypeSafe
  advisor cleanup --receipt <id> [--dry-run] [--json]
                               Release one exact advisor receipt
  advisor status [--tool <tool>] [--json]
                               List outstanding advisor receipts
  advisor provider <status|use|set-key|remove|check> [--json]
                               Configure the optional TypeSafe recommendation provider
  help                         Show this help text

Tools:
  claude
  codex
  muse
  grok
  both (all tools)
  all (all tools)
`)
}
