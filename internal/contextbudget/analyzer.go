package contextbudget

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dees91/agent-skill-manager/internal/metadata"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

const (
	defaultDiagnosticTimeout      = 6 * time.Second
	codexBudgetFraction           = 0.02
	codexUnknownBudgetCharacters  = 8000
	claudeBudgetFraction          = 0.01
	claudeUnknownContextWindow    = 200000
	claudeDefaultDescriptionLimit = 1536
	museBudgetFraction            = 0.01
	museUnknownContextWindow      = 200000
	unboundedCodexContextWindow   = 100000000
)

// Analyzer builds best-effort global catalog reports for the desktop app.
type Analyzer struct {
	paths    paths.Paths
	runner   commandRunner
	timeout  time.Duration
	lookPath func(string) (string, error)
}

// New creates an analyzer using timeout-bounded local provider commands.
func New(p paths.Paths) *Analyzer {
	lookPath := exec.LookPath
	if currentHome, err := os.UserHomeDir(); err != nil || filepath.Clean(currentHome) != filepath.Clean(p.Home) {
		lookPath = nil
	}
	return &Analyzer{paths: p, runner: execRunner{home: p.Home}, timeout: defaultDiagnosticTimeout, lookPath: lookPath}
}

// Estimate builds the applied global catalogs from local files and settings
// without launching provider executables.
func (a *Analyzer) Estimate(rows []model.SkillRow) Result {
	return a.analyze(rows, false)
}

// Measure explicitly runs the supported read-only provider diagnostics and
// falls back to the filesystem estimate when a provider is unavailable.
func (a *Analyzer) Measure(rows []model.SkillRow) Result {
	return a.analyze(rows, true)
}

func (a *Analyzer) analyze(rows []model.SkillRow, measure bool) Result {
	result := Result{contributions: make(map[CellKey]contribution)}
	type providerResult struct {
		report        ToolReport
		contributions map[CellKey]contribution
	}
	var claude, codex, muse providerResult
	var wait sync.WaitGroup
	wait.Add(3)
	go func() {
		defer wait.Done()
		claude.report, claude.contributions = a.analyzeClaude(rows, measure)
	}()
	go func() {
		defer wait.Done()
		codex.report, codex.contributions = a.analyzeCodex(rows, measure)
	}()
	go func() {
		defer wait.Done()
		muse.report, muse.contributions = a.analyzeMuse(rows)
	}()
	wait.Wait()
	result.Reports = Reports{Claude: claude.report, Codex: codex.report, Muse: muse.report}
	for key, item := range claude.contributions {
		result.contributions[key] = item
	}
	for key, item := range codex.contributions {
		result.contributions[key] = item
	}
	for key, item := range muse.contributions {
		result.contributions[key] = item
	}
	return result
}

type catalogEntry struct {
	Name        string
	Description string
	Path        string
	Line        string
}

func (entry catalogEntry) characters() int {
	return utf8.RuneCountInString(entry.Line)
}

func catalogCharacters(entries []catalogEntry) int {
	total := 0
	for _, entry := range entries {
		total += entry.characters()
	}
	if len(entries) > 1 {
		total += len(entries) - 1
	}
	return total
}

func (a *Analyzer) analyzeCodex(rows []model.SkillRow, measure bool) (ToolReport, map[CellKey]contribution) {
	modelName, contextWindow, contextAssumed := a.codexModel()
	report := ToolReport{
		Tool:                 model.ToolCodex.String(),
		Model:                modelName,
		ContextWindowTokens:  contextWindow,
		ContextWindowAssumed: contextAssumed,
		BudgetFraction:       codexBudgetFraction,
		Accuracy:             AccuracyPartial,
		Coverage:             "Known global user and system skills; provider diagnostics unavailable.",
		Message:              "Codex diagnostics are unavailable; showing a filesystem estimate.",
	}
	if contextWindow > 0 {
		report.BudgetCharacters = int(float64(contextWindow) * codexBudgetFraction * charactersPerToken)
		report.BudgetLabel = "2% of model context"
	} else {
		report.BudgetCharacters = codexUnknownBudgetCharacters
		report.BudgetLabel = "8,000-character fallback"
		report.ContextWindowAssumed = true
	}
	report.BudgetTokens = tokensForCharacters(report.BudgetCharacters)

	fallbackEntries, contributions := a.codexFilesystemEntries(rows)
	report.Current = Usage{
		SkillCount:          len(fallbackEntries),
		RequestedCharacters: catalogCharacters(fallbackEntries),
		RenderedCharacters:  min(catalogCharacters(fallbackEntries), report.BudgetCharacters),
	}

	binary, err := a.resolveBinary("codex")
	if measure && err == nil {
		actual, raw, diagnosticErr := a.codexCatalogs(binary)
		if diagnosticErr == nil && len(raw) > 0 {
			rawCharacters := catalogCharacters(raw)
			actualCharacters := catalogCharacters(actual)
			shortened, omitted := compareCatalogs(raw, actual)
			report.Current = Usage{
				SkillCount:            len(raw),
				RequestedCharacters:   rawCharacters,
				RenderedCharacters:    actualCharacters,
				ShortenedDescriptions: shortened,
				OmittedSkills:         omitted,
			}
			report.Accuracy = AccuracyMeasured
			report.Coverage = "Global Codex user, system, host, and enabled plugin skills from a neutral directory."
			report.Message = "Measured from Codex's model-visible global catalog."
			contributions = codexContributions(rows, raw)
		} else if diagnosticErr != nil {
			report.Message = "Codex diagnostics failed; showing a filesystem estimate: " + compactError(diagnosticErr)
		}
	} else if measure && err != nil {
		report.Message = "Codex diagnostics are unavailable; showing a filesystem estimate: " + compactError(err)
	} else {
		report.Message = "Filesystem estimate. Run provider diagnostics for a model-visible measurement."
	}
	finalizeUsage(&report.Current, report.BudgetCharacters)
	report.Projected = report.Current
	return report, contributions
}

func (a *Analyzer) codexCatalogs(binary string) ([]catalogEntry, []catalogEntry, error) {
	directory, err := os.MkdirTemp("", "skill-manager-codex-context-")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(directory)

	type output struct {
		data []byte
		err  error
	}
	var actualOutput, rawOutput output
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
		defer cancel()
		actualOutput.data, actualOutput.err = a.runner.Run(ctx, directory, binary, "debug", "prompt-input")
	}()
	go func() {
		defer wait.Done()
		ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
		defer cancel()
		rawOutput.data, rawOutput.err = a.runner.Run(ctx, directory, binary, "debug", "prompt-input", "-c", "model_context_window="+strconv.Itoa(unboundedCodexContextWindow))
	}()
	wait.Wait()
	if actualOutput.err != nil {
		return nil, nil, actualOutput.err
	}
	if rawOutput.err != nil {
		return nil, nil, rawOutput.err
	}
	actual, err := parseCodexPromptInput(actualOutput.data)
	if err != nil {
		return nil, nil, err
	}
	raw, err := parseCodexPromptInput(rawOutput.data)
	if err != nil {
		return nil, nil, err
	}
	return actual, raw, nil
}

func parseCodexPromptInput(data []byte) ([]catalogEntry, error) {
	var items []struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parse Codex prompt input: %w", err)
	}
	for _, item := range items {
		for _, content := range item.Content {
			if !strings.Contains(content.Text, "<skills_instructions>") {
				continue
			}
			entries := extractCodexEntries(content.Text)
			if len(entries) == 0 {
				return nil, errors.New("Codex prompt input contains no available skill entries")
			}
			return entries, nil
		}
	}
	return nil, errors.New("Codex prompt input has no skills block")
}

func extractCodexEntries(block string) []catalogEntry {
	lines := strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n")
	inAvailable := false
	entries := make([]catalogEntry, 0)
	for _, line := range lines {
		if strings.TrimSpace(line) == "### Available skills" {
			inAvailable = true
			continue
		}
		if !inAvailable {
			continue
		}
		if strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "</skills_instructions>") {
			break
		}
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		entries = append(entries, parseCodexEntry(line))
	}
	return entries
}

func parseCodexEntry(line string) catalogEntry {
	entry := catalogEntry{Line: line}
	value := strings.TrimPrefix(line, "- ")
	if index := strings.LastIndex(value, " (file: "); index >= 0 && strings.HasSuffix(value, ")") {
		entry.Path = strings.TrimSpace(value[index+8 : len(value)-1])
		value = value[:index]
	}
	if name, description, ok := strings.Cut(value, ": "); ok {
		entry.Name = strings.TrimSpace(name)
		entry.Description = strings.TrimSpace(description)
	} else {
		entry.Name = strings.TrimSpace(strings.TrimSuffix(value, ":"))
	}
	return entry
}

func compareCatalogs(raw, actual []catalogEntry) (int, int) {
	actualByName := make(map[string][]catalogEntry)
	for _, entry := range actual {
		actualByName[entry.Name] = append(actualByName[entry.Name], entry)
	}
	shortened := 0
	omitted := 0
	for _, entry := range raw {
		candidates := actualByName[entry.Name]
		if len(candidates) == 0 {
			omitted++
			continue
		}
		candidate := candidates[0]
		actualByName[entry.Name] = candidates[1:]
		if utf8.RuneCountInString(candidate.Description) < utf8.RuneCountInString(entry.Description) {
			shortened++
		}
	}
	return shortened, omitted
}

func codexContributions(rows []model.SkillRow, raw []catalogEntry) map[CellKey]contribution {
	byPath := make(map[string]catalogEntry)
	for _, entry := range raw {
		if entry.Path != "" && !strings.HasPrefix(entry.Path, "r") {
			byPath[filepath.Clean(entry.Path)] = entry
		}
	}
	result := make(map[CellKey]contribution)
	for _, row := range rows {
		cell := row.Codex
		if cell == nil || cell.ReadOnly {
			continue
		}
		key := CellKey{Tool: model.ToolCodex, SkillName: cell.Name}
		line := codexLine(cell)
		if entry, ok := byPath[filepath.Clean(cell.SkillFilePath)]; ok {
			line = entry.Line
		}
		result[key] = contribution{Characters: utf8.RuneCountInString(line) + 1, Included: cell.State == model.SkillStateOn}
	}
	return result
}

func (a *Analyzer) codexFilesystemEntries(rows []model.SkillRow) ([]catalogEntry, map[CellKey]contribution) {
	entries := make([]catalogEntry, 0)
	contributions := make(map[CellKey]contribution)
	seen := make(map[string]struct{})
	for _, row := range rows {
		cell := row.Codex
		if cell == nil || cell.ReadOnly {
			continue
		}
		line := codexLine(cell)
		key := CellKey{Tool: model.ToolCodex, SkillName: cell.Name}
		contributions[key] = contribution{Characters: utf8.RuneCountInString(line) + 1, Included: cell.State == model.SkillStateOn}
		if cell.State == model.SkillStateOn {
			entries = append(entries, parseCodexEntry(line))
			seen[cell.Name] = struct{}{}
		}
	}
	for _, item := range scanImmediateSkills(a.paths.CodexSystemSkills) {
		if _, ok := seen[item.name]; ok {
			continue
		}
		meta := metadata.ReadSkillMetadata(item.skillFile, item.name)
		line := fmt.Sprintf("- %s: %s (file: %s)", meta.Name, meta.Description, item.skillFile)
		entries = append(entries, parseCodexEntry(line))
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, contributions
}

func codexLine(cell *model.ToolSkill) string {
	name := strings.TrimSpace(cell.DisplayName)
	if name == "" {
		name = cell.Name
	}
	skillFile := cell.SkillFilePath
	if cell.State != model.SkillStateOn && cell.ActivePath != "" {
		skillFile = filepath.Join(cell.ActivePath, "SKILL.md")
	}
	return fmt.Sprintf("- %s: %s (file: %s)", name, strings.TrimSpace(cell.Description), skillFile)
}

// analyzeMuse builds a labeled filesystem estimate for the Muse global
// catalog. Muse exposes no supported read-only diagnostic in this version,
// so the report is always an estimate and never launches a subprocess.
func (a *Analyzer) analyzeMuse(rows []model.SkillRow) (ToolReport, map[CellKey]contribution) {
	budgetCharacters := int(float64(museUnknownContextWindow) * museBudgetFraction * charactersPerToken)
	report := ToolReport{
		Tool:                 model.ToolMuse.String(),
		Model:                "Muse default",
		ContextWindowTokens:  museUnknownContextWindow,
		ContextWindowAssumed: true,
		BudgetFraction:       museBudgetFraction,
		BudgetCharacters:     budgetCharacters,
		BudgetTokens:         tokensForCharacters(budgetCharacters),
		BudgetLabel:          "1% of assumed 200,000-token context",
		Accuracy:             AccuracyEstimated,
		Coverage:             "Managed Muse user skills; no provider diagnostic is available.",
		Message:              "Filesystem estimate. Muse exposes no supported catalog diagnostic in this version.",
	}
	entries := make([]catalogEntry, 0)
	contributions := make(map[CellKey]contribution)
	for _, row := range rows {
		cell := row.Muse
		if cell == nil || cell.ReadOnly {
			continue
		}
		line := museLine(cell)
		key := CellKey{Tool: model.ToolMuse, SkillName: cell.Name}
		contributions[key] = contribution{Characters: utf8.RuneCountInString(line) + 1, Included: cell.State == model.SkillStateOn}
		if cell.State == model.SkillStateOn {
			entries = append(entries, parseCodexEntry(line))
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	report.Current = Usage{
		SkillCount:          len(entries),
		RequestedCharacters: catalogCharacters(entries),
		RenderedCharacters:  min(catalogCharacters(entries), report.BudgetCharacters),
	}
	finalizeUsage(&report.Current, report.BudgetCharacters)
	report.Projected = report.Current
	return report, contributions
}

func museLine(cell *model.ToolSkill) string {
	name := strings.TrimSpace(cell.DisplayName)
	if name == "" {
		name = cell.Name
	}
	skillFile := cell.SkillFilePath
	if cell.State != model.SkillStateOn && cell.ActivePath != "" {
		skillFile = filepath.Join(cell.ActivePath, "SKILL.md")
	}
	return fmt.Sprintf("- %s: %s (file: %s)", name, strings.TrimSpace(cell.Description), skillFile)
}

func (a *Analyzer) codexModel() (string, int, bool) {
	configured := readCodexConfiguredModel(filepath.Join(a.paths.Home, ".codex", "config.toml"))
	models := []codexModel{}
	data, err := os.ReadFile(filepath.Join(a.paths.Home, ".codex", "models_cache.json"))
	if err == nil {
		models = parseCodexModels(data)
	}
	if configured != "" {
		for _, item := range models {
			if item.Slug == configured {
				return configured, item.ContextWindow, false
			}
		}
		return configured, 0, true
	}
	if len(models) > 0 {
		sort.SliceStable(models, func(i, j int) bool { return models[i].Priority < models[j].Priority })
		return models[0].Slug, models[0].ContextWindow, true
	}
	return "Codex default", 0, true
}

type codexModel struct {
	Slug          string `json:"slug"`
	ContextWindow int    `json:"context_window"`
	Priority      int    `json:"priority"`
}

func parseCodexModels(data []byte) []codexModel {
	var document struct {
		Models []codexModel `json:"models"`
	}
	if json.Unmarshal(data, &document) != nil {
		return nil
	}
	return document.Models
}

func readCodexConfiguredModel(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			break
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "model" {
			continue
		}
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			return strings.TrimSpace(unquoted)
		}
	}
	return ""
}

func (a *Analyzer) analyzeClaude(rows []model.SkillRow, measure bool) (ToolReport, map[CellKey]contribution) {
	settings := readClaudeSettings(filepath.Join(a.paths.Home, ".claude", "settings.json"))
	modelName, contextWindow, contextAssumed := claudeModelAndWindow(settings)
	budgetCharacters, fraction, label := claudeBudget(settings, contextWindow)
	report := ToolReport{
		Tool:                 model.ToolClaude.String(),
		Model:                modelName,
		ContextWindowTokens:  contextWindow,
		ContextWindowAssumed: contextAssumed,
		BudgetFraction:       fraction,
		BudgetCharacters:     budgetCharacters,
		BudgetTokens:         tokensForCharacters(budgetCharacters),
		BudgetLabel:          label,
		Accuracy:             AccuracyPartial,
		Coverage:             "Personal and enabled plugin skills; bundled, managed, and account-only catalogs may be unavailable.",
		Message:              "Claude does not expose its complete catalog outside a live session; this is a labeled local estimate.",
	}

	entries := make([]catalogEntry, 0)
	contributions := make(map[CellKey]contribution)
	for _, row := range rows {
		cell := row.Claude
		if cell == nil || cell.ReadOnly {
			continue
		}
		entry, visible := claudeCellEntry(cell, settings)
		characters := 0
		if visible {
			characters = entry.characters() + 1
		}
		contributions[CellKey{Tool: model.ToolClaude, SkillName: cell.Name}] = contribution{
			Characters: characters,
			Included:   visible && cell.State == model.SkillStateOn,
		}
		if visible && cell.State == model.SkillStateOn {
			entries = append(entries, entry)
		}
	}

	entries = append(entries, a.claudeLegacyCommands(settings)...)
	if measure {
		pluginEntries, pluginErr := a.claudePluginEntries(settings)
		entries = append(entries, pluginEntries...)
		if pluginErr != nil {
			report.Message += " Enabled plugin discovery failed: " + compactError(pluginErr)
		} else {
			report.Message = "Measured with the local Claude plugin inventory; bundled and account-only catalogs may remain unavailable."
		}
	} else {
		report.Message = "Filesystem estimate. Run provider diagnostics to include Claude's enabled plugin inventory."
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	requested := catalogCharacters(entries)
	report.Current = Usage{
		SkillCount:          len(entries),
		RequestedCharacters: requested,
		RenderedCharacters:  min(requested, budgetCharacters),
	}
	finalizeUsage(&report.Current, budgetCharacters)
	report.Projected = report.Current
	return report, contributions
}

type claudeSettings struct {
	Model                      string            `json:"model"`
	SkillListingBudgetFraction *float64          `json:"skillListingBudgetFraction"`
	SkillListingMaxDescChars   *int              `json:"skillListingMaxDescChars"`
	SkillOverrides             map[string]string `json:"skillOverrides"`
	Env                        map[string]string `json:"env"`
}

func readClaudeSettings(path string) claudeSettings {
	data, err := os.ReadFile(path)
	if err != nil {
		return claudeSettings{}
	}
	var settings claudeSettings
	_ = json.Unmarshal(data, &settings)
	return settings
}

func claudeModelAndWindow(settings claudeSettings) (string, int, bool) {
	modelName := firstNonEmpty(os.Getenv("ANTHROPIC_MODEL"), settings.Env["ANTHROPIC_MODEL"], settings.Model)
	if modelName == "" {
		modelName = "Claude default"
	}
	if configured := positiveInt(firstNonEmpty(os.Getenv("CLAUDE_CODE_MAX_CONTEXT_TOKENS"), settings.Env["CLAUDE_CODE_MAX_CONTEXT_TOKENS"])); configured > 0 {
		return modelName, configured, false
	}
	lower := strings.ToLower(modelName)
	if strings.Contains(lower, "[1m]") || strings.Contains(lower, "sonnet-5") || strings.Contains(lower, "fable-5") {
		return modelName, 1000000, false
	}
	return modelName, claudeUnknownContextWindow, true
}

func claudeBudget(settings claudeSettings, contextWindow int) (int, float64, string) {
	if fixed := positiveInt(firstNonEmpty(os.Getenv("SLASH_COMMAND_TOOL_CHAR_BUDGET"), settings.Env["SLASH_COMMAND_TOOL_CHAR_BUDGET"])); fixed > 0 {
		return fixed, 0, "fixed character budget"
	}
	fraction := claudeBudgetFraction
	if settings.SkillListingBudgetFraction != nil && *settings.SkillListingBudgetFraction > 0 {
		fraction = *settings.SkillListingBudgetFraction
	}
	return max(1, int(float64(contextWindow)*fraction*charactersPerToken)), fraction, fmt.Sprintf("%.1f%% of model context", fraction*100)
}

func claudeCellEntry(cell *model.ToolSkill, settings claudeSettings) (catalogEntry, bool) {
	meta := metadata.ReadSkillMetadata(cell.SkillFilePath, cell.Name)
	if meta.DisableModelInvocation {
		return catalogEntry{}, false
	}
	override := strings.ToLower(strings.TrimSpace(settings.SkillOverrides[cell.Name]))
	if override == "off" || override == "user-invocable-only" {
		return catalogEntry{}, false
	}
	if override == "name-only" {
		line := "- " + cell.Name
		return catalogEntry{Name: cell.Name, Path: cell.SkillFilePath, Line: line}, true
	}
	description := claudeDescription(meta, cell.SkillFilePath, settings)
	line := fmt.Sprintf("- %s: %s", cell.Name, description)
	return catalogEntry{Name: cell.Name, Description: description, Path: cell.SkillFilePath, Line: line}, true
}

func claudeDescription(meta metadata.SkillMetadata, skillFile string, settings claudeSettings) string {
	description := strings.TrimSpace(meta.Description)
	if description == "" {
		description = firstBodyParagraph(skillFile)
	}
	if meta.WhenToUse != "" {
		description = strings.TrimSpace(description + " - " + meta.WhenToUse)
	}
	limit := claudeDefaultDescriptionLimit
	if settings.SkillListingMaxDescChars != nil && *settings.SkillListingMaxDescChars > 0 {
		limit = *settings.SkillListingMaxDescChars
	}
	return truncateRunes(description, limit)
}

func (a *Analyzer) claudeLegacyCommands(settings claudeSettings) []catalogEntry {
	directory := filepath.Join(a.paths.Home, ".claude", "commands")
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	entries := make([]catalogEntry, 0)
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".md" {
			continue
		}
		name := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		override := strings.ToLower(strings.TrimSpace(settings.SkillOverrides[name]))
		if override == "off" || override == "user-invocable-only" {
			continue
		}
		path := filepath.Join(directory, file.Name())
		if override == "name-only" {
			entries = append(entries, catalogEntry{Name: name, Path: path, Line: "- " + name})
			continue
		}
		meta := metadata.ReadSkillMetadata(path, name)
		if meta.DisableModelInvocation {
			continue
		}
		description := claudeDescription(meta, path, settings)
		entries = append(entries, catalogEntry{Name: name, Description: description, Path: path, Line: fmt.Sprintf("- %s: %s", name, description)})
	}
	return entries
}

func (a *Analyzer) claudePluginEntries(settings claudeSettings) ([]catalogEntry, error) {
	binary, err := a.resolveBinary("claude")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	data, err := a.runner.Run(ctx, a.paths.Home, binary, "plugin", "list", "--json")
	if err != nil {
		return nil, err
	}
	plugins, err := parseClaudePlugins(data)
	if err != nil {
		return nil, err
	}
	entries := make([]catalogEntry, 0)
	for _, plugin := range plugins {
		if !plugin.Enabled || plugin.Path == "" {
			continue
		}
		files := scanRecursiveSkillFiles(plugin.Path)
		for _, file := range files {
			meta := metadata.ReadSkillMetadata(file, filepath.Base(filepath.Dir(file)))
			if meta.DisableModelInvocation {
				continue
			}
			name := strings.TrimSpace(meta.Name)
			if name == "" {
				name = filepath.Base(filepath.Dir(file))
			}
			name = plugin.Name + ":" + name
			override := strings.ToLower(strings.TrimSpace(settings.SkillOverrides[name]))
			if override == "off" || override == "user-invocable-only" {
				continue
			}
			if override == "name-only" {
				entries = append(entries, catalogEntry{Name: name, Path: file, Line: "- " + name})
				continue
			}
			description := claudeDescription(meta, file, settings)
			entries = append(entries, catalogEntry{Name: name, Description: description, Path: file, Line: fmt.Sprintf("- %s: %s", name, description)})
		}
	}
	return entries, nil
}

type claudePlugin struct {
	Name    string
	Path    string
	Enabled bool
}

func parseClaudePlugins(data []byte) ([]claudePlugin, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse Claude plugin list: %w", err)
	}
	list, ok := raw.([]any)
	if !ok {
		if object, objectOK := raw.(map[string]any); objectOK {
			list, _ = object["plugins"].([]any)
		}
	}
	plugins := make([]claudePlugin, 0, len(list))
	for _, value := range list {
		record, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name := stringField(record, "id", "name", "plugin")
		path := stringField(record, "installPath", "install_path", "path")
		status := strings.ToLower(stringField(record, "status"))
		enabled := !strings.Contains(status, "disabled")
		if value, exists := record["enabled"].(bool); exists {
			enabled = value
		}
		if at := strings.Index(name, "@"); at >= 0 {
			name = name[:at]
		}
		if name == "" && path != "" {
			name = filepath.Base(path)
		}
		plugins = append(plugins, claudePlugin{Name: name, Path: path, Enabled: enabled})
	}
	return plugins, nil
}

type discoveredSkill struct {
	name      string
	skillFile string
}

func scanImmediateSkills(directory string) []discoveredSkill {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	result := make([]discoveredSkill, 0)
	for _, file := range files {
		if !file.IsDir() && file.Type()&os.ModeSymlink == 0 {
			continue
		}
		skillFile := filepath.Join(directory, file.Name(), "SKILL.md")
		if info, err := os.Stat(skillFile); err == nil && info.Mode().IsRegular() {
			result = append(result, discoveredSkill{name: file.Name(), skillFile: skillFile})
		}
	}
	return result
}

func scanRecursiveSkillFiles(root string) []string {
	ignored := map[string]struct{}{".git": {}, "node_modules": {}, ".venv": {}, "vendor": {}, "build": {}, "dist": {}}
	files := make([]string, 0)
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if _, skip := ignored[entry.Name()]; skip && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "SKILL.md" {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func firstBodyParagraph(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for index := 1; index < len(lines); index++ {
			if strings.TrimSpace(lines[index]) == "---" {
				start = index + 1
				break
			}
		}
	}
	paragraph := make([]string, 0)
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(paragraph) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") && len(paragraph) == 0 {
			continue
		}
		paragraph = append(paragraph, trimmed)
	}
	return strings.Join(paragraph, " ")
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func (a *Analyzer) resolveBinary(name string) (string, error) {
	candidates := []string{filepath.Join(a.paths.Home, ".local", "bin", name)}
	if name == "codex" {
		candidates = append(candidates, filepath.Join(a.paths.Home, ".codex", "packages", "standalone", "current", "bin", "codex"))
	} else if name == "claude" {
		candidates = append(candidates, filepath.Join(a.paths.Home, ".local", "share", "claude", "versions", "current"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if a.lookPath != nil {
		if resolved, err := a.lookPath(name); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("%s executable not found", name)
}

func compactError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.Join(strings.Fields(err.Error()), " ")
	return truncateRunes(message, 180)
}

func positiveInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringField(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := record[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
