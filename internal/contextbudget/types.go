// Package contextbudget measures the model-visible global skill catalog without
// changing provider configuration or skill state.
package contextbudget

import (
	"math"

	"github.com/dees91/agent-skill-manager/internal/model"
)

const charactersPerToken = 4

// Accuracy describes how directly a provider report was measured.
type Accuracy string

const (
	AccuracyMeasured  Accuracy = "measured"
	AccuracyEstimated Accuracy = "estimated"
	AccuracyPartial   Accuracy = "partial"
)

// Health is the budget pressure state used by the dashboard.
type Health string

const (
	HealthOK          Health = "ok"
	HealthNearLimit   Health = "near-limit"
	HealthOverBudget  Health = "over-budget"
	HealthUnavailable Health = "unavailable"
)

// Usage is one applied or projected catalog measurement.
type Usage struct {
	SkillCount            int     `json:"skillCount"`
	RequestedCharacters   int     `json:"requestedCharacters"`
	RenderedCharacters    int     `json:"renderedCharacters"`
	EstimatedTokens       int     `json:"estimatedTokens"`
	RenderedTokens        int     `json:"renderedTokens"`
	UsedPercent           float64 `json:"usedPercent"`
	ShortenedDescriptions int     `json:"shortenedDescriptions"`
	OmittedSkills         int     `json:"omittedSkills"`
	Health                Health  `json:"health"`
}

// ToolReport is the complete dashboard report for one provider.
type ToolReport struct {
	Tool                 string   `json:"tool"`
	Model                string   `json:"model"`
	ContextWindowTokens  int      `json:"contextWindowTokens"`
	ContextWindowAssumed bool     `json:"contextWindowAssumed"`
	BudgetFraction       float64  `json:"budgetFraction"`
	BudgetCharacters     int      `json:"budgetCharacters"`
	BudgetTokens         int      `json:"budgetTokens"`
	BudgetLabel          string   `json:"budgetLabel"`
	Accuracy             Accuracy `json:"accuracy"`
	Coverage             string   `json:"coverage"`
	Message              string   `json:"message"`
	Current              Usage    `json:"current"`
	Projected            Usage    `json:"projected"`
	ProjectionChanged    bool     `json:"projectionChanged"`
}

// Reports contains the two provider rows shown by the Dashboard.
type Reports struct {
	Claude ToolReport `json:"claude"`
	Codex  ToolReport `json:"codex"`
}

// CellKey is a path-free managed skill cell identity.
type CellKey struct {
	Tool      model.Tool
	SkillName string
}

type contribution struct {
	Characters int
	Included   bool
}

// Result stores an applied report plus managed-cell contributions used to
// project pending changes without re-running provider diagnostics.
type Result struct {
	Reports       Reports
	contributions map[CellKey]contribution
}

// Project applies pending operations to the cached report in memory.
func (r Result) Project(pending map[CellKey]model.OperationKind) Reports {
	reports := r.Reports
	for key, operation := range pending {
		entry, ok := r.contributions[key]
		if !ok || entry.Characters <= 0 {
			continue
		}
		report := &reports.Claude
		if key.Tool == model.ToolCodex {
			report = &reports.Codex
		}
		switch operation {
		case model.OperationDisable:
			if entry.Included {
				report.Projected.RequestedCharacters -= entry.Characters
				report.Projected.SkillCount--
				report.ProjectionChanged = true
			}
		case model.OperationEnable:
			if !entry.Included {
				report.Projected.RequestedCharacters += entry.Characters
				report.Projected.SkillCount++
				report.ProjectionChanged = true
			}
		}
	}
	recalculateProjected(&reports.Claude)
	recalculateProjected(&reports.Codex)
	return reports
}

func recalculateProjected(report *ToolReport) {
	if !report.ProjectionChanged {
		report.Projected = report.Current
		return
	}
	usage := &report.Projected
	if usage.RequestedCharacters < 0 {
		usage.RequestedCharacters = 0
	}
	if usage.SkillCount < 0 {
		usage.SkillCount = 0
	}
	usage.EstimatedTokens = tokensForCharacters(usage.RequestedCharacters)
	usage.RenderedCharacters = usage.RequestedCharacters
	if report.BudgetCharacters > 0 && usage.RenderedCharacters > report.BudgetCharacters {
		usage.RenderedCharacters = report.BudgetCharacters
	}
	usage.RenderedTokens = tokensForCharacters(usage.RenderedCharacters)
	usage.UsedPercent = percent(usage.RequestedCharacters, report.BudgetCharacters)
	usage.ShortenedDescriptions = 0
	usage.OmittedSkills = 0
	usage.Health = healthFor(usage.UsedPercent, report.BudgetCharacters > 0)
}

func finalizeUsage(usage *Usage, budgetCharacters int) {
	usage.EstimatedTokens = tokensForCharacters(usage.RequestedCharacters)
	usage.RenderedTokens = tokensForCharacters(usage.RenderedCharacters)
	usage.UsedPercent = percent(usage.RequestedCharacters, budgetCharacters)
	usage.Health = healthFor(usage.UsedPercent, budgetCharacters > 0)
}

func tokensForCharacters(characters int) int {
	if characters <= 0 {
		return 0
	}
	return int(math.Ceil(float64(characters) / charactersPerToken))
}

func percent(value, total int) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(value)/float64(total)*1000) / 10
}

func healthFor(usedPercent float64, available bool) Health {
	if !available {
		return HealthUnavailable
	}
	switch {
	case usedPercent > 100:
		return HealthOverBudget
	case usedPercent >= 80:
		return HealthNearLimit
	default:
		return HealthOK
	}
}
