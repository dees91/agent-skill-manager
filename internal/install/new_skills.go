package install

import "github.com/dees91/agent-skill-manager/internal/state"

// NewSkills returns skills discovered in the repository checkout whose names
// are not recorded as installed. Matching is by name, so a recorded skill that
// moved to another path in the checkout is not reported as new.
func NewSkills(repository state.RepositoryEntry) ([]DiscoveredSkill, error) {
	discovered, err := DiscoverSkills(repository.CheckoutPath)
	if err != nil {
		return nil, err
	}
	recorded := make(map[string]bool, len(repository.InstalledSkills))
	for _, skill := range repository.InstalledSkills {
		recorded[skill.Name] = true
	}
	newSkills := []DiscoveredSkill{}
	for _, skill := range discovered {
		if !recorded[skill.Name] {
			newSkills = append(newSkills, skill)
		}
	}
	return newSkills, nil
}
