package gui

import (
	"errors"
	"fmt"
	"time"

	"github.com/dees91/agent-skill-manager/internal/advisor"
)

// ListAdvisorReceipts returns every recorded advisor receipt with the actions a
// cleanup would take, from a dry-run of each (Iteration 28). A receipt whose
// dry-run stops is blocked and carries a path-free cause. It mutates nothing.
func (s *Service) ListAdvisorReceipts() ([]AdvisorReceipt, error) {
	service := advisor.New(s.paths)
	status, err := service.Status(nil)
	if err != nil {
		return nil, errors.New("Skill Manager could not read the advisor receipts.")
	}
	receipts := make([]AdvisorReceipt, 0, len(status.Receipts))
	for _, current := range status.Receipts {
		receipt := AdvisorReceipt{ReceiptID: current.ReceiptID, Tool: current.Tool.String(), CreatedAt: current.CreatedAt.UTC().Format(time.RFC3339), Skills: []AdvisorReceiptSkill{}}
		preview, previewErr := service.Cleanup(current.ReceiptID, true)
		if previewErr != nil {
			receipt.Blocked = true
			receipt.Cause = advisorCause(previewErr)
			for _, name := range current.Skills {
				receipt.Skills = append(receipt.Skills, AdvisorReceiptSkill{Name: name})
			}
		} else {
			for _, action := range preview.Actions {
				receipt.Skills = append(receipt.Skills, AdvisorReceiptSkill{Name: action.Skill, Action: action.Action})
			}
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

// CleanupAdvisorReceipt releases one receipt, like advisor cleanup.
func (s *Service) CleanupAdvisorReceipt(receiptID string, includeReadOnly bool) AdvisorReceiptResult {
	return s.runAdvisorReceiptOperation(includeReadOnly, func(result *AdvisorReceiptResult) error {
		if _, err := advisor.New(s.paths).Cleanup(receiptID, false); err != nil {
			result.Failure = &AdvisorReceiptFailure{ReceiptID: receiptID, Message: advisorCause(err)}
			return err
		}
		result.Cleaned = append(result.Cleaned, receiptID)
		result.Message = "Cleaned up 1 receipt."
		return nil
	})
}

// CleanupAllAdvisorReceipts releases every receipt that is not blocked, in list
// order, and stops at the first failure. Blocked receipts stay recorded.
func (s *Service) CleanupAllAdvisorReceipts(includeReadOnly bool) AdvisorReceiptResult {
	return s.runAdvisorReceiptOperation(includeReadOnly, func(result *AdvisorReceiptResult) error {
		receipts, err := s.ListAdvisorReceipts()
		if err != nil {
			return err
		}
		service := advisor.New(s.paths)
		skipped := 0
		for _, receipt := range receipts {
			if receipt.Blocked {
				skipped++
				continue
			}
			if _, err := service.Cleanup(receipt.ReceiptID, false); err != nil {
				result.Failure = &AdvisorReceiptFailure{ReceiptID: receipt.ReceiptID, Message: advisorCause(err)}
				return err
			}
			result.Cleaned = append(result.Cleaned, receipt.ReceiptID)
		}
		result.Message = fmt.Sprintf("Cleaned up %d receipt(s); %d blocked receipt(s) left.", len(result.Cleaned), skipped)
		return nil
	})
}

// ForgetAdvisorReceipt drops a blocked receipt without touching skill links.
func (s *Service) ForgetAdvisorReceipt(receiptID string, includeReadOnly bool) AdvisorReceiptResult {
	return s.runAdvisorReceiptOperation(includeReadOnly, func(result *AdvisorReceiptResult) error {
		if _, err := advisor.New(s.paths).Forget(receiptID, false); err != nil {
			result.Failure = &AdvisorReceiptFailure{ReceiptID: receiptID, Message: advisorCause(err)}
			return err
		}
		result.Forgotten = append(result.Forgotten, receiptID)
		result.Message = "Forgot 1 receipt. Skill links were not changed."
		return nil
	})
}

// runAdvisorReceiptOperation runs a receipt mutation in the exclusive source
// lane, then reloads the snapshot and the receipt list.
func (s *Service) runAdvisorReceiptOperation(includeReadOnly bool, action func(*AdvisorReceiptResult) error) AdvisorReceiptResult {
	result := AdvisorReceiptResult{Cleaned: []string{}, Forgotten: []string{}, Receipts: []AdvisorReceipt{}}
	err := s.runSourceOperation("advisor", "", func() error { return action(&result) })
	if err != nil && result.Failure == nil {
		// Only lane and list errors reach here; both are fixed, path-free text.
		result.Failure = &AdvisorReceiptFailure{Message: err.Error()}
	}
	if result.Failure != nil && result.Message == "" {
		result.Message = "The receipt operation failed."
	}
	s.mu.Lock()
	reloadErr := s.reloadLocked(includeReadOnly)
	result.Snapshot = s.snapshotLocked()
	s.mu.Unlock()
	if reloadErr != nil && result.Failure == nil {
		result.Failure = &AdvisorReceiptFailure{Message: "Skill Manager could not rescan the skills."}
		result.Message = "The operation finished, but the follow-up scan failed."
	}
	if receipts, listErr := s.ListAdvisorReceipts(); listErr == nil {
		result.Receipts = receipts
	}
	return result
}

// advisorCause explains a failed receipt operation without filesystem paths.
// Only classified advisor errors keep their detail; raw filesystem and
// scan errors map to a fixed message, since they can name any directory.
func advisorCause(err error) string {
	var blocked advisor.CleanupBlockedError
	switch {
	case errors.As(err, &blocked):
		return blocked.Cause()
	case errors.Is(err, advisor.ErrReceiptNotFound):
		return "The receipt is no longer recorded."
	case errors.Is(err, advisor.ErrReceiptNotBlocked):
		return "The receipt is not blocked; clean it up instead."
	default:
		return unclassifiedAdvisorFailure
	}
}

const unclassifiedAdvisorFailure = "Skill Manager could not read or change the skills of this receipt. Run skill-manager advisor cleanup --receipt <receipt-id> --dry-run in a terminal for details."
