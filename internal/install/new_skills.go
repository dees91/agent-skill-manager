package install

import "github.com/dees91/agent-skill-manager/internal/state"

// NewSkillsReport lists skills discovered in a managed checkout whose names
// are not recorded as installed. Skills carries unique and identical-copies
// skills resolved to one copy; Ambiguous carries conflicting groups that need
// an explicit --skill <name>=<path> choice before they can be installed.
type NewSkillsReport struct {
	Skills    []DiscoveredSkill
	Ambiguous []AmbiguousGroup
}

// NewSkills returns skills discovered in the repository checkout whose names
// are not recorded as installed. Matching is by name, so a recorded skill
// that moved to another path in the checkout is not reported as new. Only
// checkout inspection failures are errors; ambiguous groups are reported.
func NewSkills(repository state.RepositoryEntry) (NewSkillsReport, error) {
	discovered, err := DiscoverSkills(repository.CheckoutPath)
	if err != nil {
		return NewSkillsReport{}, err
	}
	recorded := make(map[string]bool, len(repository.InstalledSkills))
	for _, skill := range repository.InstalledSkills {
		recorded[skill.Name] = true
	}
	report := NewSkillsReport{Skills: []DiscoveredSkill{}, Ambiguous: []AmbiguousGroup{}}
	for _, skill := range discovered.Skills {
		if !recorded[skill.Name] {
			report.Skills = append(report.Skills, skill)
		}
	}
	for _, group := range discovered.Groups {
		if recorded[group.Name] {
			continue
		}
		if group.Identical {
			report.Skills = append(report.Skills, group.Candidates[0])
			continue
		}
		report.Ambiguous = append(report.Ambiguous, ambiguousGroupFromDuplicate(group))
	}
	return report, nil
}
