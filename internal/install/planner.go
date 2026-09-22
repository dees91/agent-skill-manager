package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// ToolTarget is the install CLI target before it expands to concrete tools.
type ToolTarget string

const (
	ToolTargetClaude ToolTarget = "claude"
	ToolTargetCodex  ToolTarget = "codex"
	ToolTargetMuse   ToolTarget = "muse"
	ToolTargetGrok   ToolTarget = "grok"
	ToolTargetBoth   ToolTarget = "both"
	ToolTargetAll    ToolTarget = "all"
)

// ParseToolTarget expands an install target into concrete tools. Empty, both,
// and all default to every supported tool.
func ParseToolTarget(value string) ([]model.Tool, error) {
	switch ToolTarget(strings.TrimSpace(value)) {
	case "":
		return model.Tools(), nil
	case ToolTargetBoth:
		return model.Tools(), nil
	case ToolTargetAll:
		return model.Tools(), nil
	case ToolTargetClaude:
		return []model.Tool{model.ToolClaude}, nil
	case ToolTargetCodex:
		return []model.Tool{model.ToolCodex}, nil
	case ToolTargetMuse:
		return []model.Tool{model.ToolMuse}, nil
	case ToolTargetGrok:
		return []model.Tool{model.ToolGrok}, nil
	default:
		return nil, fmt.Errorf("invalid tool target %q", value)
	}
}

// PlanOptions controls install plan selection.
type PlanOptions struct {
	Tools      []model.Tool
	SkillNames []string
	Cells      []InstallCell
	// Off plans new links directly at the tool's disabled path, so the skill
	// never appears in the active skills directory until it is enabled.
	Off bool
	// InstalledAs maps a source skill name to the link basename it is
	// installed under when its plain name is owned by another source.
	InstalledAs map[string]string
}

// InstallCell identifies one exact skill/tool target. When Cells is non-empty,
// it replaces the Cartesian Tools x SkillNames selection used by the CLI.
// Path optionally qualifies the skill with a source-relative copy when one
// name was discovered at several paths. InstalledAs optionally names the
// link basename when it differs from the source skill name.
type InstallCell struct {
	SkillName   string
	Tool        model.Tool
	Path        string
	InstalledAs string
}

type selectedInstallCell struct {
	Skill       DiscoveredSkill
	Tool        model.Tool
	InstalledAs string
}

func (c selectedInstallCell) installedName() string {
	return installedNameOf(c.Skill, c.InstalledAs)
}

func installedNameOf(skill DiscoveredSkill, installedAs string) string {
	if installedAs != "" {
		return installedAs
	}
	return skill.Name
}

// LinkPlan is one symlink the installer should create. TargetPath is always
// the active skills path; DisabledPath is set when the link is created OFF.
// InstalledAs is set when the link basename differs from the skill name.
type LinkPlan struct {
	Skill        DiscoveredSkill
	Tool         model.Tool
	TargetPath   string
	DisabledPath string
	InstalledAs  string
}

// InstalledName is the link basename in the tool directory.
func (l LinkPlan) InstalledName() string {
	return installedNameOf(l.Skill, l.InstalledAs)
}

// LinkPath is where the symlink is created on disk.
func (l LinkPlan) LinkPath() string {
	if l.DisabledPath != "" {
		return l.DisabledPath
	}
	return l.TargetPath
}

// AlreadyInstalled records an idempotent install preflight result.
type AlreadyInstalled struct {
	Skill        DiscoveredSkill
	Tool         model.Tool
	TargetPath   string
	DisabledPath string
	State        model.SkillState
	InstalledAs  string
}

// InstalledName is the link basename in the tool directory.
func (a AlreadyInstalled) InstalledName() string {
	return installedNameOf(a.Skill, a.InstalledAs)
}

// InstallPlan is a side-effect-free plan for installing repository skills.
type InstallPlan struct {
	Identity           RepoIdentity
	CheckoutPath       string
	Group              model.GroupLabel
	Links              []LinkPlan
	AlreadyInstalled   []AlreadyInstalled
	Off                bool
	ResolvedDuplicates []ResolvedDuplicate
}

// PreflightConflict records a target that blocks install planning.
// SuggestedAs is a free installed name offered when the plain skill name is
// owned by another source; OwnerGroup labels that source without a path.
type PreflightConflict struct {
	SkillName   string
	Tool        model.Tool
	TargetPath  string
	Reason      string
	Existing    string
	Expected    string
	Disabled    string
	Description string
	SuggestedAs string
	OwnerGroup  model.GroupLabel
}

// PlanError reports all install preflight failures in deterministic order.
type PlanError struct {
	MissingSkills []string
	Conflicts     []PreflightConflict
}

func (e PlanError) Error() string {
	var parts []string
	if len(e.MissingSkills) > 0 {
		parts = append(parts, "missing skills: "+strings.Join(e.MissingSkills, ", "))
	}
	if len(e.Conflicts) > 0 {
		conflicts := make([]string, len(e.Conflicts))
		for i, conflict := range e.Conflicts {
			conflicts[i] = fmt.Sprintf("%s/%s at %s: %s", conflict.Tool, conflict.SkillName, conflict.TargetPath, conflict.Reason)
		}
		parts = append(parts, "conflicts: "+strings.Join(conflicts, "; "))
	}
	if len(parts) == 0 {
		return "install plan failed"
	}
	return strings.Join(parts, "; ")
}

// PlanInstall builds and preflights a side-effect-free repository install plan.
func PlanInstall(p paths.Paths, manifest state.Manifest, identity RepoIdentity, checkoutPath string, discovered Discovery, options PlanOptions) (InstallPlan, error) {
	if err := validatePlanIdentity(identity); err != nil {
		return InstallPlan{}, err
	}
	checkoutPath = strings.TrimSpace(checkoutPath)
	if checkoutPath == "" {
		return InstallPlan{}, fmt.Errorf("checkout path is required")
	}
	var err error
	checkoutPath, err = filepath.Abs(filepath.Clean(checkoutPath))
	if err != nil {
		return InstallPlan{}, fmt.Errorf("resolve checkout path: %w", err)
	}

	recorded := RecordedGitSkillPaths(manifest, identity.Host, identity.RepoPath)
	recordedNames := RecordedGitInstalledNames(manifest, identity.Host, identity.RepoPath)
	selectedCells, resolution, err := selectInstallCells(p, checkoutPath, discovered, options, recorded, recordedNames)
	if err != nil {
		return InstallPlan{}, err
	}
	missingSkills := resolution.Missing
	suggest := ownershipSuggester(p, manifest, discovered, selectedCells, recordedNames, GitSourceRef(identity))

	plan := InstallPlan{
		Identity:           identity,
		CheckoutPath:       checkoutPath,
		Group:              identity.Group,
		Links:              []LinkPlan{},
		Off:                options.Off,
		ResolvedDuplicates: resolution.Resolved,
	}
	plan.AlreadyInstalled = []AlreadyInstalled{}
	var conflicts []PreflightConflict

	if len(missingSkills) == 0 {
		for _, selected := range selectedCells {
			skill, tool := selected.Skill, selected.Tool
			if owner := gitInstallCellOwner(manifest, tool, selected.installedName(), identity); owner.found() {
				conflicts = append(conflicts, suggest(selected, owner))
				continue
			}
			link, already, conflict := planSkillToolWithOptions(p, manifest, skill, tool, selected.InstalledAs, options.Off)
			if conflict != nil {
				conflicts = append(conflicts, *conflict)
				continue
			}
			if already != nil {
				plan.AlreadyInstalled = append(plan.AlreadyInstalled, *already)
				continue
			}
			if link != nil {
				plan.Links = append(plan.Links, *link)
			}
		}
	}

	if len(missingSkills) > 0 || len(conflicts) > 0 {
		sort.Strings(missingSkills)
		sortConflicts(conflicts)
		return InstallPlan{}, PlanError{MissingSkills: missingSkills, Conflicts: conflicts}
	}
	return plan, nil
}

func selectInstallCells(p paths.Paths, root string, discovered Discovery, options PlanOptions, recorded, recordedNames map[string]string) ([]selectedInstallCell, Resolution, error) {
	explicitNames, err := explicitInstalledNames(options)
	if err != nil {
		return nil, Resolution{}, err
	}
	if len(options.Cells) == 0 {
		tools, err := normalizePlanTools(p, options.Tools)
		if err != nil {
			return nil, Resolution{}, err
		}
		resolution, err := selectDiscoveredSkills(root, discovered, options.SkillNames, recorded)
		if err != nil {
			return nil, Resolution{}, err
		}
		aliases, err := resolveSelectionInstalledNames(discovered, resolution, explicitNames, recordedNames)
		if err != nil {
			return nil, Resolution{}, err
		}
		cells := make([]selectedInstallCell, 0, len(resolution.Selected)*len(tools))
		for _, skill := range resolution.Selected {
			for _, tool := range tools {
				cells = append(cells, selectedInstallCell{Skill: skill, Tool: tool, InstalledAs: aliases[skill.Name]})
			}
		}
		return cells, resolution, nil
	}
	if len(options.Tools) > 0 || len(options.SkillNames) > 0 {
		return nil, Resolution{}, fmt.Errorf("exact install cells cannot be combined with tools or skill names")
	}
	if len(options.Cells) == 0 {
		return nil, Resolution{}, fmt.Errorf("at least one install cell is required")
	}
	names, explicit, err := CellsToRequests(options.Cells)
	if err != nil {
		return nil, Resolution{}, err
	}
	resolution, err := ResolveDiscovery(discovered, names, explicit, recorded)
	if err != nil {
		return nil, Resolution{}, err
	}
	byName := make(map[string]DiscoveredSkill, len(resolution.Selected))
	for _, skill := range resolution.Selected {
		normalized, err := validateDiscoveredSkill(root, skill)
		if err != nil {
			return nil, Resolution{}, err
		}
		byName[normalized.Name] = normalized
	}
	aliases, err := resolveSelectionInstalledNames(discovered, resolution, explicitNames, recordedNames)
	if err != nil {
		return nil, Resolution{}, err
	}
	seen := map[string]bool{}
	cells := make([]selectedInstallCell, 0, len(options.Cells))
	for _, requested := range options.Cells {
		name := strings.TrimSpace(requested.SkillName)
		if _, ok := p.UserSkillsDirFor(requested.Tool); !ok {
			return nil, Resolution{}, fmt.Errorf("invalid install tool %q", requested.Tool)
		}
		skill, ok := byName[name]
		if !ok {
			continue
		}
		key := repositoryCellKey(requested.Tool, name)
		if seen[key] {
			continue
		}
		seen[key] = true
		cells = append(cells, selectedInstallCell{Skill: skill, Tool: requested.Tool, InstalledAs: aliases[name]})
	}
	sort.SliceStable(cells, func(i, j int) bool {
		if cells[i].Skill.Name != cells[j].Skill.Name {
			return cells[i].Skill.Name < cells[j].Skill.Name
		}
		return toolRank(cells[i].Tool) < toolRank(cells[j].Tool)
	})
	if len(cells) == 0 && len(resolution.Missing) == 0 {
		return nil, Resolution{}, fmt.Errorf("at least one install cell is required")
	}
	return cells, resolution, nil
}

// explicitInstalledNames merges PlanOptions.InstalledAs with per-cell
// InstalledAs values; one skill name maps to at most one installed name.
func explicitInstalledNames(options PlanOptions) (map[string]string, error) {
	explicit := map[string]string{}
	add := func(name, installed string) error {
		name, installed = strings.TrimSpace(name), strings.TrimSpace(installed)
		if name == "" || installed == "" {
			return fmt.Errorf("install name mapping needs a skill name and an install name")
		}
		if previous, ok := explicit[name]; ok && previous != installed {
			return fmt.Errorf("conflicting install names for skill %q: %q and %q", name, previous, installed)
		}
		explicit[name] = installed
		return nil
	}
	for name, installed := range options.InstalledAs {
		if err := add(name, installed); err != nil {
			return nil, err
		}
	}
	for _, cell := range options.Cells {
		if strings.TrimSpace(cell.InstalledAs) == "" {
			continue
		}
		if err := add(cell.SkillName, cell.InstalledAs); err != nil {
			return nil, err
		}
	}
	return explicit, nil
}

// resolveSelectionInstalledNames skips alias resolution while skills are
// missing, so the missing-skill report is not masked by --as errors.
func resolveSelectionInstalledNames(discovered Discovery, resolution Resolution, explicit, recordedNames map[string]string) (map[string]string, error) {
	if len(resolution.Missing) > 0 {
		return map[string]string{}, nil
	}
	selected := make([]string, len(resolution.Selected))
	for i, skill := range resolution.Selected {
		selected[i] = skill.Name
	}
	return resolveInstalledNames(discovered, selected, explicit, recordedNames)
}

// ownershipSuggester builds the conflict for a cell owned by another source.
// A suggestion is offered only when the plain name was requested and this
// source has no record for the skill; a recorded plain name gets guidance
// instead, because changing a recorded name needs uninstall plus reinstall.
func ownershipSuggester(p paths.Paths, manifest state.Manifest, discovered Discovery, cells []selectedInstallCell, recordedNames map[string]string, source SourceRef) func(selectedInstallCell, cellOwner) PreflightConflict {
	aliases := map[string]string{}
	toolSet := map[model.Tool]bool{}
	tools := []model.Tool{}
	for _, cell := range cells {
		if cell.InstalledAs != "" {
			aliases[cell.Skill.Name] = cell.InstalledAs
		}
		if !toolSet[cell.Tool] {
			toolSet[cell.Tool] = true
			tools = append(tools, cell.Tool)
		}
	}
	var taken func(string) bool
	suggested := map[string]string{}
	return func(cell selectedInstallCell, owner cellOwner) PreflightConflict {
		activeDir, _ := p.UserSkillsDirFor(cell.Tool)
		conflict := PreflightConflict{
			SkillName:  cell.Skill.Name,
			Tool:       cell.Tool,
			TargetPath: filepath.Join(activeDir, cell.installedName()),
			Reason:     "cell is already owned by " + owner.String(),
			Expected:   cell.Skill.Path,
			OwnerGroup: owner.group,
		}
		_, recorded := recordedNames[cell.Skill.Name]
		switch {
		case recorded:
			conflict.Reason += "; uninstall this source and reinstall it with --as, or install only the other tools"
		case cell.InstalledAs == "":
			if _, ok := suggested[cell.Skill.Name]; !ok {
				if taken == nil {
					taken = installedNameTaken(p, manifest, discovered, aliases, tools)
				}
				name := SuggestInstalledName(cell.Skill.Name, source, func(candidate string) bool {
					if taken(candidate) {
						return true
					}
					for _, other := range suggested {
						if other == candidate {
							return true
						}
					}
					return false
				})
				suggested[cell.Skill.Name] = name
			}
			conflict.SuggestedAs = suggested[cell.Skill.Name]
		}
		return conflict
	}
}

func validatePlanIdentity(identity RepoIdentity) error {
	if strings.TrimSpace(identity.OriginalURL) == "" {
		return fmt.Errorf("repository URL is required")
	}
	if strings.TrimSpace(identity.CanonicalURL) == "" ||
		strings.TrimSpace(identity.Host) == "" ||
		strings.TrimSpace(identity.RepoPath) == "" {
		return fmt.Errorf("repository identity is incomplete")
	}
	return nil
}

func normalizePlanTools(p paths.Paths, tools []model.Tool) ([]model.Tool, error) {
	if len(tools) == 0 {
		tools = model.Tools()
	}
	seen := map[model.Tool]bool{}
	normalized := make([]model.Tool, 0, len(tools))
	for _, tool := range tools {
		if _, ok := p.UserSkillsDirFor(tool); !ok {
			return nil, fmt.Errorf("invalid install tool %q", tool)
		}
		if seen[tool] {
			continue
		}
		seen[tool] = true
		normalized = append(normalized, tool)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return toolRank(normalized[i]) < toolRank(normalized[j])
	})
	return normalized, nil
}

func toolRank(tool model.Tool) int {
	for i, known := range model.Tools() {
		if tool == known {
			return i
		}
	}
	return len(model.Tools()) + 1
}

func selectDiscoveredSkills(checkoutPath string, discovered Discovery, requested []string, recorded map[string]string) (Resolution, error) {
	if discovered.Empty() && len(requested) == 0 {
		return Resolution{}, fmt.Errorf("no installable skills discovered")
	}

	names, explicit, err := SplitSkillRequests(discovered, requested)
	if err != nil {
		return Resolution{}, err
	}
	resolution, err := ResolveDiscovery(discovered, names, explicit, recorded)
	if err != nil {
		return Resolution{}, err
	}
	for i, skill := range resolution.Selected {
		normalized, err := validateDiscoveredSkill(checkoutPath, skill)
		if err != nil {
			return Resolution{}, err
		}
		resolution.Selected[i] = normalized
	}
	return resolution, nil
}

func validateDiscoveredSkill(checkoutPath string, skill DiscoveredSkill) (DiscoveredSkill, error) {
	if strings.TrimSpace(skill.Name) == "" {
		return DiscoveredSkill{}, fmt.Errorf("discovered skill has empty name")
	}
	if skill.Name != strings.TrimSpace(skill.Name) || filepath.Base(skill.Name) != skill.Name || skill.Name == "." || skill.Name == ".." {
		return DiscoveredSkill{}, fmt.Errorf("discovered skill name %q is not a valid basename", skill.Name)
	}
	if strings.TrimSpace(skill.Path) == "" {
		return DiscoveredSkill{}, fmt.Errorf("discovered skill %q has empty path", skill.Name)
	}
	if !filepath.IsAbs(skill.Path) {
		return DiscoveredSkill{}, fmt.Errorf("discovered skill %q path must be absolute", skill.Name)
	}
	skill.Path = filepath.Clean(skill.Path)
	rel, err := filepath.Rel(checkoutPath, skill.Path)
	if err != nil {
		return DiscoveredSkill{}, fmt.Errorf("discovered skill %q path is not relative to checkout: %w", skill.Name, err)
	}
	if rel != "." && (rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel)) {
		return DiscoveredSkill{}, fmt.Errorf("discovered skill %q path escapes checkout", skill.Name)
	}
	return skill, nil
}

func planSkillTool(p paths.Paths, manifest state.Manifest, skill DiscoveredSkill, tool model.Tool, installedAs string) (*LinkPlan, *AlreadyInstalled, *PreflightConflict) {
	installedName := installedNameOf(skill, installedAs)
	activeDir, _ := p.UserSkillsDirFor(tool)
	targetPath := filepath.Join(activeDir, installedName)
	expectedTarget := filepath.Clean(skill.Path)

	info, err := os.Lstat(targetPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(targetPath)
			if err != nil {
				return nil, nil, conflict(skill, tool, targetPath, "cannot read existing symlink", "", expectedTarget, "", err.Error())
			}
			if samePath(resolveLinkTarget(targetPath, target), expectedTarget) {
				return nil, &AlreadyInstalled{
					Skill:       skill,
					Tool:        tool,
					TargetPath:  targetPath,
					State:       model.SkillStateOn,
					InstalledAs: installedAs,
				}, nil
			}
			return nil, nil, conflict(skill, tool, targetPath, "target symlink points elsewhere", resolveLinkTarget(targetPath, target), expectedTarget, "", "")
		}
		existing := "file"
		if info.IsDir() {
			existing = "directory"
		}
		return nil, nil, conflict(skill, tool, targetPath, "target path already exists", existing, expectedTarget, "", "")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, conflict(skill, tool, targetPath, "cannot inspect target path", "", expectedTarget, "", err.Error())
	}

	if entry, ok := manifest.Get(tool, installedName); ok {
		disabledTarget := resolveLinkTarget(entry.OriginalPath, entry.SymlinkTarget)
		if entry.EntryType == model.EntryTypeSymlink && samePath(disabledTarget, expectedTarget) {
			return nil, &AlreadyInstalled{
				Skill:        skill,
				Tool:         tool,
				TargetPath:   targetPath,
				DisabledPath: entry.DisabledPath,
				State:        model.SkillStateOff,
				InstalledAs:  installedAs,
			}, nil
		}
		return nil, nil, conflict(skill, tool, targetPath, "disabled state points elsewhere", disabledTarget, expectedTarget, entry.DisabledPath, "")
	}

	return &LinkPlan{Skill: skill, Tool: tool, TargetPath: targetPath, InstalledAs: installedAs}, nil, nil
}

// planSkillToolWithOptions plans one cell and, for an OFF install, moves a new
// link to the disabled path after checking that path is free.
func planSkillToolWithOptions(p paths.Paths, manifest state.Manifest, skill DiscoveredSkill, tool model.Tool, installedAs string, off bool) (*LinkPlan, *AlreadyInstalled, *PreflightConflict) {
	link, already, blocked := planSkillTool(p, manifest, skill, tool, installedAs)
	if link == nil || !off {
		return link, already, blocked
	}
	disabledPath, err := state.New(p).DisabledPath(tool, link.InstalledName())
	if err != nil {
		return nil, nil, conflict(skill, tool, link.TargetPath, "cannot resolve disabled path", "", link.Skill.Path, "", err.Error())
	}
	info, err := os.Lstat(disabledPath)
	if err == nil {
		existing := "file"
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			existing = "symlink"
		case info.IsDir():
			existing = "directory"
		}
		return nil, nil, conflict(skill, tool, link.TargetPath, "disabled path already exists", existing, link.Skill.Path, disabledPath, "")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, conflict(skill, tool, link.TargetPath, "cannot inspect disabled path", "", link.Skill.Path, disabledPath, err.Error())
	}
	link.DisabledPath = disabledPath
	return link, nil, nil
}

func conflict(skill DiscoveredSkill, tool model.Tool, targetPath, reason, existing, expected, disabled, description string) *PreflightConflict {
	return &PreflightConflict{
		SkillName:   skill.Name,
		Tool:        tool,
		TargetPath:  targetPath,
		Reason:      reason,
		Existing:    existing,
		Expected:    expected,
		Disabled:    disabled,
		Description: description,
	}
}

func resolveLinkTarget(linkPath, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(linkPath), target))
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(filepath.Clean(left))
	rightAbs, rightErr := filepath.Abs(filepath.Clean(right))
	if leftErr == nil && rightErr == nil {
		return leftAbs == rightAbs
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func sortConflicts(conflicts []PreflightConflict) {
	sort.SliceStable(conflicts, func(i, j int) bool {
		if conflicts[i].Tool != conflicts[j].Tool {
			return conflicts[i].Tool.String() < conflicts[j].Tool.String()
		}
		if conflicts[i].SkillName != conflicts[j].SkillName {
			return conflicts[i].SkillName < conflicts[j].SkillName
		}
		return conflicts[i].TargetPath < conflicts[j].TargetPath
	})
}
