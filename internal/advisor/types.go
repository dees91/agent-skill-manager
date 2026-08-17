// Package advisor owns temporary, receipt-scoped skill activations.
package advisor

import (
	"fmt"
	"sort"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
)

const (
	// APIVersion is the public JSON contract used by first-party advisor skills.
	APIVersion  = 1
	fileVersion = 1
	// MaxSkillsPerActivation bounds model-selected mutations.
	MaxSkillsPerActivation = 5
)

// Action describes one activation or cleanup decision.
type Action struct {
	Skill  string `json:"skill"`
	Action string `json:"action"`
}

// ActivateResult is returned by advisor activate.
type ActivateResult struct {
	APIVersion int        `json:"apiVersion"`
	DryRun     bool       `json:"dryRun"`
	ReceiptID  string     `json:"receiptId,omitempty"`
	Tool       model.Tool `json:"tool"`
	Actions    []Action   `json:"actions"`
}

// CleanupResult is returned by advisor cleanup.
type CleanupResult struct {
	APIVersion int        `json:"apiVersion"`
	DryRun     bool       `json:"dryRun"`
	ReceiptID  string     `json:"receiptId"`
	Tool       model.Tool `json:"tool"`
	Actions    []Action   `json:"actions"`
}

// ReceiptStatus is the public, path-free view of one outstanding receipt.
type ReceiptStatus struct {
	ReceiptID string     `json:"receiptId"`
	Tool      model.Tool `json:"tool"`
	CreatedAt time.Time  `json:"createdAt"`
	Skills    []string   `json:"skills"`
}

// StatusResult is returned by advisor status.
type StatusResult struct {
	APIVersion int             `json:"apiVersion"`
	Receipts   []ReceiptStatus `json:"receipts"`
}

type file struct {
	Version  int       `json:"version"`
	Receipts []receipt `json:"receipts"`
	Leases   []lease   `json:"leases"`
}

type receipt struct {
	ID        string     `json:"id"`
	Tool      model.Tool `json:"tool"`
	CreatedAt time.Time  `json:"createdAt"`
	Skills    []string   `json:"skills"`
}

type lease struct {
	Tool          model.Tool      `json:"tool"`
	SkillName     string          `json:"skillName"`
	OriginalPath  string          `json:"originalPath"`
	DisabledPath  string          `json:"disabledPath"`
	EntryType     model.EntryType `json:"entryType"`
	SymlinkTarget string          `json:"symlinkTarget,omitempty"`
	ReceiptIDs    []string        `json:"receiptIds"`
}

func emptyFile() file {
	return file{Version: fileVersion, Receipts: []receipt{}, Leases: []lease{}}
}

func (f file) receiptIndex(id string) int {
	for index := range f.Receipts {
		if f.Receipts[index].ID == id {
			return index
		}
	}
	return -1
}

func (f file) leaseIndex(tool model.Tool, skillName string) int {
	for index := range f.Leases {
		if f.Leases[index].Tool == tool && f.Leases[index].SkillName == skillName {
			return index
		}
	}
	return -1
}

func (f *file) removeClaim(receiptID string, tool model.Tool, skillName string) error {
	receiptIndex := f.receiptIndex(receiptID)
	leaseIndex := f.leaseIndex(tool, skillName)
	if receiptIndex < 0 || leaseIndex < 0 {
		return fmt.Errorf("advisor claim %s %s/%s is incomplete", receiptID, tool, skillName)
	}
	f.Receipts[receiptIndex].Skills = removeString(f.Receipts[receiptIndex].Skills, skillName)
	f.Leases[leaseIndex].ReceiptIDs = removeString(f.Leases[leaseIndex].ReceiptIDs, receiptID)
	if len(f.Leases[leaseIndex].ReceiptIDs) == 0 {
		f.Leases = append(f.Leases[:leaseIndex], f.Leases[leaseIndex+1:]...)
	}
	if len(f.Receipts[receiptIndex].Skills) == 0 {
		f.Receipts = append(f.Receipts[:receiptIndex], f.Receipts[receiptIndex+1:]...)
	}
	f.normalizeOrder()
	return nil
}

func (f *file) normalizeOrder() {
	if f.Receipts == nil {
		f.Receipts = []receipt{}
	}
	if f.Leases == nil {
		f.Leases = []lease{}
	}
	for index := range f.Receipts {
		sort.Strings(f.Receipts[index].Skills)
	}
	for index := range f.Leases {
		sort.Strings(f.Leases[index].ReceiptIDs)
	}
	sort.SliceStable(f.Receipts, func(i, j int) bool {
		if !f.Receipts[i].CreatedAt.Equal(f.Receipts[j].CreatedAt) {
			return f.Receipts[i].CreatedAt.Before(f.Receipts[j].CreatedAt)
		}
		return f.Receipts[i].ID < f.Receipts[j].ID
	})
	sort.SliceStable(f.Leases, func(i, j int) bool {
		if f.Leases[i].Tool != f.Leases[j].Tool {
			return f.Leases[i].Tool < f.Leases[j].Tool
		}
		return f.Leases[i].SkillName < f.Leases[j].SkillName
	})
}

func removeString(values []string, target string) []string {
	for index, value := range values {
		if value == target {
			return append(values[:index], values[index+1:]...)
		}
	}
	return values
}
