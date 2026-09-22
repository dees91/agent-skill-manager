package install

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/scan"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// managedClassifier labels a created link the way a managed scan would.
type managedClassifier func(manifest state.Manifest) func(tool model.Tool, name, linkPath string) (model.SourceLabel, model.GroupLabel)

func defaultManagedClassifier(p paths.Paths) managedClassifier {
	return scan.New(p).ManagedSymlinkClassifier
}

// createPlannedLinks creates every planned link at its LinkPath. Parents inside
// the Skill Manager state directory are private, matching disable.
func createPlannedLinks(p paths.Paths, links []LinkPlan, symlink func(string, string) error) ([]LinkPlan, error) {
	created := []LinkPlan{}
	for _, link := range links {
		parent := filepath.Dir(link.LinkPath())
		mode := os.FileMode(0o755)
		if pathInside(p.StateDir, parent) {
			mode = 0o700
		}
		if err := os.MkdirAll(parent, mode); err != nil {
			return created, fmt.Errorf("create symlink parent %s: %w", parent, err)
		}
		if err := symlink(link.Skill.Path, link.LinkPath()); err != nil {
			return created, fmt.Errorf("create symlink %s -> %s: %w", link.LinkPath(), link.Skill.Path, err)
		}
		created = append(created, link)
	}
	return created, nil
}

// addDisabledRecords records every link created OFF with the same entry a
// manual disable of an active link would write.
func addDisabledRecords(manifest *state.Manifest, links []LinkPlan, classify managedClassifier, now time.Time) {
	var labels func(tool model.Tool, name, linkPath string) (model.SourceLabel, model.GroupLabel)
	for _, link := range links {
		if link.DisabledPath == "" {
			continue
		}
		if labels == nil {
			labels = classify(*manifest)
		}
		source, group := labels(link.Tool, link.InstalledName(), link.DisabledPath)
		manifest.Upsert(state.DisabledEntry{
			Tool:          link.Tool,
			SkillName:     link.InstalledName(),
			OriginalPath:  link.TargetPath,
			DisabledPath:  link.DisabledPath,
			EntryType:     model.EntryTypeSymlink,
			SymlinkTarget: link.Skill.Path,
			Source:        source,
			Group:         group,
			DisabledAt:    now,
		})
	}
}

func rollbackCreated(created []LinkPlan) []LinkPlan {
	rolledBack := []LinkPlan{}
	for i := len(created) - 1; i >= 0; i-- {
		link := created[i]
		path := link.LinkPath()
		target, err := os.Readlink(path)
		if err != nil {
			continue
		}
		if !samePath(resolveLinkTarget(path, target), link.Skill.Path) {
			continue
		}
		if err := os.Remove(path); err == nil {
			rolledBack = append(rolledBack, link)
		}
	}
	return rolledBack
}
