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
}

// InstallCell identifies one exact skill/tool target. When Cells is non-empty,
// it replaces the Cartesian Tools x SkillNames selection used by the CLI.
type InstallCell struct {
	SkillName string
	Tool      model.Tool
}

type selectedInstallCell struct {
	Skill DiscoveredSkill
	Tool  model.Tool
}

// LinkPlan is one symlink the installer should create.
type LinkPlan struct {
	Skill      DiscoveredSkill
	Tool       model.Tool
	TargetPath string
}

// AlreadyInstalled records an idempotent install preflight result.
type AlreadyInstalled struct {
	Skill        DiscoveredSkill
	Tool         model.Tool
	TargetPath   string
	DisabledPath string
	State        model.SkillState
}

// InstallPlan is a side-effect-free plan for installing repository skills.
type InstallPlan struct {
	Identity         RepoIdentity
	CheckoutPath     string
	Group            model.GroupLabel
	Links            []LinkPlan
	AlreadyInstalled []AlreadyInstalled
}

// PreflightConflict records a target that blocks install planning.
type PreflightConflict struct {
	SkillName   string
	Tool        model.Tool
	TargetPath  string
	Reason      string
	Existing    string
	Expected    string
	Disabled    string
	Description string
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
func PlanInstall(p paths.Paths, manifest state.Manifest, identity RepoIdentity, checkoutPath string, discovered []DiscoveredSkill, options PlanOptions) (InstallPlan, error) {
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

	selectedCells, missingSkills, err := selectInstallCells(p, checkoutPath, discovered, options)
	if err != nil {
		return InstallPlan{}, err
	}

	plan := InstallPlan{
		Identity:     identity,
		CheckoutPath: checkoutPath,
		Group:        identity.Group,
		Links:        []LinkPlan{},
	}
	plan.AlreadyInstalled = []AlreadyInstalled{}
	var conflicts []PreflightConflict

	if len(missingSkills) == 0 {
		for _, selected := range selectedCells {
			skill, tool := selected.Skill, selected.Tool
			if owner := gitInstallCellOwner(manifest, tool, skill.Name, identity); owner != "" {
				activeDir, _ := p.UserSkillsDirFor(tool)
				conflicts = append(conflicts, PreflightConflict{
					SkillName:  skill.Name,
					Tool:       tool,
					TargetPath: filepath.Join(activeDir, skill.Name),
					Reason:     "cell is already owned by " + owner,
					Expected:   skill.Path,
				})
				continue
			}
			link, already, conflict := planSkillTool(p, manifest, skill, tool)
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

func selectInstallCells(p paths.Paths, root string, discovered []DiscoveredSkill, options PlanOptions) ([]selectedInstallCell, []string, error) {
	if len(options.Cells) == 0 {
		tools, err := normalizePlanTools(p, options.Tools)
		if err != nil {
			return nil, nil, err
		}
		skills, missing, err := selectDiscoveredSkills(root, discovered, options.SkillNames)
		if err != nil {
			return nil, nil, err
		}
		cells := make([]selectedInstallCell, 0, len(skills)*len(tools))
		for _, skill := range skills {
			for _, tool := range tools {
				cells = append(cells, selectedInstallCell{Skill: skill, Tool: tool})
			}
		}
		return cells, missing, nil
	}
	if len(options.Tools) > 0 || len(options.SkillNames) > 0 {
		return nil, nil, fmt.Errorf("exact install cells cannot be combined with tools or skill names")
	}
	byName := make(map[string]DiscoveredSkill, len(discovered))
	for _, skill := range discovered {
		normalized, err := validateDiscoveredSkill(root, skill)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := byName[normalized.Name]; exists {
			return nil, nil, fmt.Errorf("duplicate discovered skill name %q", normalized.Name)
		}
		byName[normalized.Name] = normalized
	}
	seen := map[string]bool{}
	missingSet := map[string]bool{}
	cells := make([]selectedInstallCell, 0, len(options.Cells))
	for _, requested := range options.Cells {
		name := strings.TrimSpace(requested.SkillName)
		if name == "" {
			return nil, nil, fmt.Errorf("selected skill name is required")
		}
		if _, ok := p.UserSkillsDirFor(requested.Tool); !ok {
			return nil, nil, fmt.Errorf("invalid install tool %q", requested.Tool)
		}
		skill, ok := byName[name]
		if !ok {
			missingSet[name] = true
			continue
		}
		key := repositoryCellKey(requested.Tool, name)
		if seen[key] {
			continue
		}
		seen[key] = true
		cells = append(cells, selectedInstallCell{Skill: skill, Tool: requested.Tool})
	}
	sort.SliceStable(cells, func(i, j int) bool {
		if cells[i].Skill.Name != cells[j].Skill.Name {
			return cells[i].Skill.Name < cells[j].Skill.Name
		}
		return toolRank(cells[i].Tool) < toolRank(cells[j].Tool)
	})
	missing := make([]string, 0, len(missingSet))
	for name := range missingSet {
		missing = append(missing, name)
	}
	sort.Strings(missing)
	if len(cells) == 0 && len(missing) == 0 {
		return nil, nil, fmt.Errorf("at least one install cell is required")
	}
	return cells, missing, nil
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

func selectDiscoveredSkills(checkoutPath string, discovered []DiscoveredSkill, requested []string) ([]DiscoveredSkill, []string, error) {
	if len(discovered) == 0 && len(requested) == 0 {
		return nil, nil, fmt.Errorf("no installable skills discovered")
	}

	byName := map[string]DiscoveredSkill{}
	for _, skill := range discovered {
		normalized, err := validateDiscoveredSkill(checkoutPath, skill)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := byName[normalized.Name]; exists {
			return nil, nil, fmt.Errorf("duplicate discovered skill name %q", normalized.Name)
		}
		byName[normalized.Name] = normalized
	}

	if len(requested) == 0 {
		selected := make([]DiscoveredSkill, 0, len(discovered))
		for _, skill := range discovered {
			selected = append(selected, byName[skill.Name])
		}
		return selected, nil, nil
	}

	requestedSet := map[string]bool{}
	for _, name := range requested {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, nil, fmt.Errorf("selected skill name is required")
		}
		requestedSet[name] = true
	}

	var missing []string
	for name := range requestedSet {
		if _, ok := byName[name]; !ok {
			missing = append(missing, name)
		}
	}

	selected := make([]DiscoveredSkill, 0, len(requestedSet))
	for _, skill := range discovered {
		if requestedSet[skill.Name] {
			selected = append(selected, byName[skill.Name])
		}
	}
	return selected, missing, nil
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

func planSkillTool(p paths.Paths, manifest state.Manifest, skill DiscoveredSkill, tool model.Tool) (*LinkPlan, *AlreadyInstalled, *PreflightConflict) {
	activeDir, _ := p.UserSkillsDirFor(tool)
	targetPath := filepath.Join(activeDir, skill.Name)
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
					Skill:      skill,
					Tool:       tool,
					TargetPath: targetPath,
					State:      model.SkillStateOn,
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

	if entry, ok := manifest.Get(tool, skill.Name); ok {
		disabledTarget := resolveLinkTarget(entry.OriginalPath, entry.SymlinkTarget)
		if entry.EntryType == model.EntryTypeSymlink && samePath(disabledTarget, expectedTarget) {
			return nil, &AlreadyInstalled{
				Skill:        skill,
				Tool:         tool,
				TargetPath:   targetPath,
				DisabledPath: entry.DisabledPath,
				State:        model.SkillStateOff,
			}, nil
		}
		return nil, nil, conflict(skill, tool, targetPath, "disabled state points elsewhere", disabledTarget, expectedTarget, entry.DisabledPath, "")
	}

	return &LinkPlan{Skill: skill, Tool: tool, TargetPath: targetPath}, nil, nil
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
