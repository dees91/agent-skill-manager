// Package advisor owns temporary, receipt-scoped skill activations.
package advisor

import (
	"errors"
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
	// CapabilitySemanticRecommendation identifies optional TypeSafe recommendations.
	CapabilitySemanticRecommendation = "semantic_recommendation_v1"
)

// Capabilities returns the stable additive features supported by this CLI.
func Capabilities() []string {
	return []string{CapabilityRankedSearch, CapabilitySemanticRecommendation}
}

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

// ForgetResult is returned by advisor forget.
type ForgetResult struct {
	APIVersion int        `json:"apiVersion"`
	DryRun     bool       `json:"dryRun"`
	ReceiptID  string     `json:"receiptId"`
	Tool       model.Tool `json:"tool"`
	Skills     []string   `json:"skills"`
	Cause      string     `json:"cause,omitempty"`
}

// Reasons a receipt cleanup is blocked.
const (
	BlockedDrift    = "drift"
	BlockedConflict = "conflict"
	BlockedMissing  = "missing"
)

var (
	// ErrReceiptNotFound reports a receipt ID that is not recorded.
	ErrReceiptNotFound = errors.New("advisor receipt not found")
	// ErrReceiptNotBlocked reports a forget request for a receipt that cleanup
	// can release.
	ErrReceiptNotBlocked = errors.New("advisor receipt is not blocked")
)

// receiptError keeps the historical message and matches its sentinel.
type receiptError struct {
	kind    error
	message string
}

func (e receiptError) Error() string { return e.message }
func (e receiptError) Unwrap() error { return e.kind }

// CleanupBlockedError reports a receipt whose cleanup would touch an entry
// that no longer matches its lease. Error keeps the historical message; Cause
// is a path-free explanation for interfaces.
type CleanupBlockedError struct {
	Tool    model.Tool
	Skill   string
	Reason  string
	message string
}

func blockedError(reason string, tool model.Tool, skill, message string) CleanupBlockedError {
	return CleanupBlockedError{Tool: tool, Skill: skill, Reason: reason, message: message}
}

func (e CleanupBlockedError) Error() string { return e.message }

// Cause explains the block by tool and skill name only.
func (e CleanupBlockedError) Cause() string {
	switch e.Reason {
	case BlockedMissing:
		return fmt.Sprintf("%s/%s is no longer installed where the advisor enabled it.", e.Tool, e.Skill)
	case BlockedConflict:
		return fmt.Sprintf("%s/%s has both an active and a disabled entry.", e.Tool, e.Skill)
	default:
		return fmt.Sprintf("%s/%s changed after the advisor enabled it.", e.Tool, e.Skill)
	}
}

// ReceiptStatus is the public, path-free view of one outstanding receipt.
type ReceiptStatus struct {
	ReceiptID string     `json:"receiptId"`
	Tool      model.Tool `json:"tool"`
	CreatedAt time.Time  `json:"createdAt"`
	Skills    []string   `json:"skills"`
}

// ProviderStatus is the path-free configured recommendation provider.
type ProviderStatus struct {
	Mode    ProviderMode `json:"mode"`
	Warning string       `json:"warning,omitempty"`
}

// StatusResult is returned by advisor status.
type StatusResult struct {
	APIVersion   int             `json:"apiVersion"`
	Capabilities []string        `json:"capabilities"`
	Receipts     []ReceiptStatus `json:"receipts"`
	Provider     ProviderStatus  `json:"provider"`
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

// forgetReceipt drops one receipt and its claims, and every lease left
// without claims. It never touches the filesystem.
func (f *file) forgetReceipt(receiptIndex int) {
	id := f.Receipts[receiptIndex].ID
	f.Receipts = append(f.Receipts[:receiptIndex], f.Receipts[receiptIndex+1:]...)
	kept := f.Leases[:0]
	for _, current := range f.Leases {
		current.ReceiptIDs = removeString(current.ReceiptIDs, id)
		if len(current.ReceiptIDs) > 0 {
			kept = append(kept, current)
		}
	}
	f.Leases = kept
	f.normalizeOrder()
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
