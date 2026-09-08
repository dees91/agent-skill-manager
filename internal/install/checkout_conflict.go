package install

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// CheckoutConflictKind classifies why a managed checkout cannot be updated,
// uninstalled, or repaired. Kinds are stable identifiers safe to project into
// the CLI and the desktop bridge.
type CheckoutConflictKind string

// Checkout conflict kinds.
const (
	CheckoutConflictSymlink         CheckoutConflictKind = "symlink"
	CheckoutConflictNotDirectory    CheckoutConflictKind = "not-directory"
	CheckoutConflictNotCheckout     CheckoutConflictKind = "not-a-checkout"
	CheckoutConflictNestedCheckout  CheckoutConflictKind = "nested-checkout"
	CheckoutConflictIdentity        CheckoutConflictKind = "identity"
	CheckoutConflictOriginMissing   CheckoutConflictKind = "origin-missing"
	CheckoutConflictOriginMismatch  CheckoutConflictKind = "origin-mismatch"
	CheckoutConflictDirtyWorktree   CheckoutConflictKind = "dirty-worktree"
	CheckoutConflictDetachedHead    CheckoutConflictKind = "detached-head"
	CheckoutConflictNoUpstream      CheckoutConflictKind = "no-upstream"
	CheckoutConflictForeignUpstream CheckoutConflictKind = "foreign-upstream"
	CheckoutConflictUnresolvedRef   CheckoutConflictKind = "unresolved-ref"
	CheckoutConflictDiverged        CheckoutConflictKind = "diverged"
)

// Worktree entry classes reported by a dirty-worktree conflict.
const (
	WorktreeEntryTracked   = "tracked"
	WorktreeEntryUntracked = "untracked"
	WorktreeEntryIgnored   = "ignored"
)

// WorktreeEntry is one checkout-relative path that makes a managed checkout
// dirty. Paths are always relative to the checkout root.
type WorktreeEntry struct {
	RelPath string
	Class   string
	Status  string
	IsDir   bool
}

// CheckoutConflictError reports a classified managed checkout blocker. Message
// preserves the historical error text exactly; Kind carries the classification.
type CheckoutConflictError struct {
	Kind    CheckoutConflictKind
	Path    string
	Message string
	Entries []WorktreeEntry
	Err     error
}

// Error returns the unchanged historical conflict message.
func (e CheckoutConflictError) Error() string { return e.Message }

// Unwrap exposes the underlying cause for wrapped conflicts.
func (e CheckoutConflictError) Unwrap() error { return e.Err }

// Repairable reports whether repair can clear this conflict. Only a dirty
// worktree is repairable; every other kind needs a manual decision.
func (e CheckoutConflictError) Repairable() bool {
	return e.Kind == CheckoutConflictDirtyWorktree
}

// Cause returns a short, path-free description of the blocker.
func (e CheckoutConflictError) Cause() string {
	switch e.Kind {
	case CheckoutConflictSymlink:
		return "The managed checkout is a symlink instead of a real directory."
	case CheckoutConflictNotDirectory:
		return "The managed checkout path is not a directory."
	case CheckoutConflictNotCheckout:
		return "The managed checkout is not a Git checkout."
	case CheckoutConflictNestedCheckout:
		return "The managed checkout is nested inside another Git checkout."
	case CheckoutConflictIdentity:
		return "The recorded repository identity is incomplete."
	case CheckoutConflictOriginMissing:
		return "The managed checkout has no origin remote."
	case CheckoutConflictOriginMismatch:
		return "The managed checkout points at a different origin remote."
	case CheckoutConflictDirtyWorktree:
		return dirtyWorktreeCause(e.Entries)
	case CheckoutConflictDetachedHead:
		return "The managed checkout is in detached HEAD state."
	case CheckoutConflictNoUpstream:
		return "The current branch does not track an origin branch."
	case CheckoutConflictForeignUpstream:
		return "The current branch tracks an upstream outside origin."
	case CheckoutConflictUnresolvedRef:
		return "A required Git reference could not be resolved."
	case CheckoutConflictDiverged:
		return "The managed checkout has local commits and cannot fast-forward."
	default:
		return "The managed checkout cannot be used."
	}
}

// Remedy returns the recommended next step for conflicts repair cannot fix.
func (e CheckoutConflictError) Remedy() string {
	switch e.Kind {
	case CheckoutConflictDirtyWorktree:
		return "Repair stages these paths into the Skill Manager trash and restores the checkout."
	case CheckoutConflictDetachedHead, CheckoutConflictNoUpstream, CheckoutConflictForeignUpstream:
		return "Check out the tracking branch again, or uninstall and reinstall the repository."
	case CheckoutConflictDiverged:
		return "Keep or discard the local commits yourself, then update again."
	case CheckoutConflictOriginMismatch, CheckoutConflictOriginMissing:
		return "Restore the origin remote, or uninstall and reinstall the repository."
	case CheckoutConflictSymlink, CheckoutConflictNotDirectory, CheckoutConflictNotCheckout, CheckoutConflictNestedCheckout:
		return "Move the blocking path aside yourself, then reinstall the repository."
	default:
		return "Resolve the checkout state manually, then try again."
	}
}

func dirtyWorktreeCause(entries []WorktreeEntry) string {
	if len(entries) == 0 {
		return "The managed checkout has tracked, untracked, or ignored worktree changes."
	}
	counts := CountWorktreeEntries(entries)
	parts := []string{}
	if counts[WorktreeEntryTracked] > 0 {
		parts = append(parts, fmt.Sprintf("%d tracked", counts[WorktreeEntryTracked]))
	}
	if counts[WorktreeEntryUntracked] > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", counts[WorktreeEntryUntracked]))
	}
	if counts[WorktreeEntryIgnored] > 0 {
		parts = append(parts, fmt.Sprintf("%d ignored", counts[WorktreeEntryIgnored]))
	}
	noun := "paths"
	if len(entries) == 1 {
		noun = "path"
	}
	return fmt.Sprintf("The managed checkout has %d worktree %s (%s).", len(entries), noun, strings.Join(parts, ", "))
}

// CountWorktreeEntries counts entries per class.
func CountWorktreeEntries(entries []WorktreeEntry) map[string]int {
	counts := map[string]int{WorktreeEntryTracked: 0, WorktreeEntryUntracked: 0, WorktreeEntryIgnored: 0}
	for _, entry := range entries {
		counts[entry.Class]++
	}
	return counts
}

// AsCheckoutConflict extracts the typed checkout conflict carried by err.
func AsCheckoutConflict(err error) (CheckoutConflictError, bool) {
	var conflict CheckoutConflictError
	if errors.As(err, &conflict) {
		return conflict, true
	}
	return CheckoutConflictError{}, false
}

// conflictf builds a typed conflict whose message matches the historical text
// exactly. A %w verb in format is preserved as the wrapped cause.
func conflictf(kind CheckoutConflictKind, path, format string, args ...any) CheckoutConflictError {
	wrapped := fmt.Errorf(format, args...)
	return CheckoutConflictError{Kind: kind, Path: path, Message: wrapped.Error(), Err: errors.Unwrap(wrapped)}
}

// dirtyWorktreeConflict builds the dirty-worktree conflict, attaching the exact
// offending entries when git can still report them.
func dirtyWorktreeConflict(checkoutPath string, runner GitRunner) CheckoutConflictError {
	conflict := conflictf(CheckoutConflictDirtyWorktree, checkoutPath, "checkout conflict: %s has tracked, untracked, or ignored worktree changes", checkoutPath)
	status, err := runner.RunGit("-C", checkoutPath, "status", "--porcelain=v2", "-z", "--untracked-files=all", "--ignored")
	if err != nil {
		return conflict
	}
	conflict.Entries = parseWorktreeEntries(status)
	return conflict
}

// Porcelain v2 status prefixes for untracked and ignored records.
const (
	worktreeStatusUntracked = "?"
	worktreeStatusIgnored   = "!"
)

// parseWorktreeEntries parses NUL-separated `git status --porcelain=v2 -z`
// records. Version 2 is required rather than v1: every v2 record starts with
// `1`, `2`, `u`, `?`, or `!`, so the output can never begin with whitespace.
// A v1 record may start with the space of an "X " status code, which the
// trimming GitRunner would silently remove, making it indistinguishable from a
// filename that begins with a space. Path bytes are preserved exactly; only the
// documented trailing directory slash is stripped.
func parseWorktreeEntries(output string) []WorktreeEntry {
	records := strings.Split(output, "\x00")
	entries := []WorktreeEntry{}
	seen := map[string]bool{}
	add := func(status, path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		entries = append(entries, WorktreeEntry{
			RelPath: strings.TrimSuffix(path, "/"),
			Class:   worktreeEntryClass(status),
			Status:  status,
			IsDir:   strings.HasSuffix(path, "/"),
		})
	}
	for index := 0; index < len(records); index++ {
		record := records[index]
		if len(record) < 3 || record[1] != ' ' {
			continue
		}
		switch record[0] {
		case '?':
			add(worktreeStatusUntracked, record[2:])
		case '!':
			add(worktreeStatusIgnored, record[2:])
		case '1':
			if status, path, ok := splitStatusRecord(record, 8); ok {
				add(status, path)
			}
		case '2':
			status, path, ok := splitStatusRecord(record, 9)
			if !ok {
				continue
			}
			add(status, path)
			// A rename or copy carries its original path as the next record.
			if index+1 < len(records) {
				index++
				add(status, records[index])
			}
		case 'u':
			if status, path, ok := splitStatusRecord(record, 10); ok {
				add(status, path)
			}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].RelPath < entries[j].RelPath })
	return entries
}

// splitStatusRecord returns the XY field and the path of a porcelain v2 record
// with the given number of space-separated fields before the path. The path is
// never split, so spaces inside it are preserved.
func splitStatusRecord(record string, fields int) (string, string, bool) {
	parts := strings.SplitN(record, " ", fields+1)
	if len(parts) != fields+1 {
		return "", "", false
	}
	return parts[1], parts[fields], true
}

func worktreeEntryClass(status string) string {
	switch status {
	case worktreeStatusUntracked:
		return WorktreeEntryUntracked
	case worktreeStatusIgnored:
		return WorktreeEntryIgnored
	default:
		return WorktreeEntryTracked
	}
}
