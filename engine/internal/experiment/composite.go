package experiment

import (
	"math"
	"strings"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

// computeComposite blends code/judge/process/spec-adherence on a 0..10 scale.
// Harness-adherence is intentionally EXCLUDED (separate diagnostic).
//
// Formula (all on 0..10 scale):
//   - code  = grade.TestPassRate * 10
//   - judge = mean of grade.JudgeScores, EXCLUDING any dim whose rationale
//             starts with "judge_unavailable". If no eligible dims → 0.
//   - process = ((ranValidation?1:0)*0.4 + (1-grade.ToolErrorRate)*0.4 + tokenEfficiency*0.2) * 10
//               where tokenEfficiency = clamp(0,1, 20000 / max(TotalTokens, 20000))
//   - spec  = grade.SpecInstructionCompliance * 10
//   - composite = round4(code*0.3 + judge*0.3 + process*0.2 + spec*0.2)
func computeComposite(grade models.Grade) float64 {
	// Code term (0..10).
	code := grade.TestPassRate * 10

	// Judge term (0..10): average non-unavailable dimensions.
	judge := judgeScore(grade)

	// Process term (0..10).
	process := processScore(grade)

	// Spec term (0..10).
	spec := grade.SpecInstructionCompliance * 10

	composite := code*0.3 + judge*0.3 + process*0.2 + spec*0.2
	return round4(composite)
}

// judgeScore computes the judge term from the grade's JudgeScores map,
// excluding any dimension whose rationale begins with "judge_unavailable".
// Returns 0 when no eligible dimensions exist.
func judgeScore(grade models.Grade) float64 {
	if len(grade.JudgeScores) == 0 {
		return 0
	}
	var sum float64
	var count int
	for dim, score := range grade.JudgeScores {
		if rationale, ok := grade.JudgeRationales[dim]; ok {
			if strings.HasPrefix(rationale, "judge_unavailable") {
				continue
			}
		}
		sum += score
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// processScore computes the process term (0..10) from RanValidation,
// ToolErrorRate, and TotalTokens.
func processScore(grade models.Grade) float64 {
	var validationScore float64
	if grade.RanValidation {
		validationScore = 1.0
	}
	toolReliability := 1 - grade.ToolErrorRate
	tokenEfficiency := clamp01(20000.0 / math.Max(float64(grade.TotalTokens), 20000.0))
	return (validationScore*0.4 + toolReliability*0.4 + tokenEfficiency*0.2) * 10
}

// clamp01 clamps v to the range [0, 1].
func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

// round4 rounds v to 4 decimal places.
func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
