package experiment

import (
	"math"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

func TestComputeComposite_NormalBlend(t *testing.T) {
	grade := models.Grade{
		TestPassRate:              0.8,  // code = 8.0
		JudgeScores:               map[string]float64{"correctness": 7.0, "clarity": 9.0},
		RanValidation:             true,
		ToolErrorRate:             0.1,
		TotalTokens:               10000, // tokenEfficiency = 20000/20000 clamped? no: 20000/10000=2.0 → clamp → 1.0
		SpecInstructionCompliance: 0.9,
	}
	// code = 0.8 * 10 = 8.0
	// judge = (7 + 9) / 2 = 8.0
	// tokenEfficiency = clamp(0,1, 20000.0/max(10000,20000)) = clamp(0,1,20000/20000) = 1.0
	// process = (1*0.4 + (1-0.1)*0.4 + 1.0*0.2) * 10 = (0.4 + 0.36 + 0.2) * 10 = 9.6
	// spec = 0.9 * 10 = 9.0
	// composite = 8.0*0.3 + 8.0*0.3 + 9.6*0.2 + 9.0*0.2
	//           = 2.4 + 2.4 + 1.92 + 1.8 = 8.52
	got := computeComposite(grade)
	want := 8.52
	if math.Abs(got-want) > 1e-3 {
		t.Errorf("computeComposite normal blend = %v, want %v", got, want)
	}
}

func TestComputeComposite_JudgeUnavailableDimExcluded(t *testing.T) {
	grade := models.Grade{
		TestPassRate: 1.0,
		JudgeScores: map[string]float64{
			"correctness": 8.0,
			"clarity":     6.0,
		},
		JudgeRationales: map[string]string{
			"correctness": "judge_unavailable: no api key",
			"clarity":     "Great clarity in the solution.",
		},
		RanValidation:             false,
		ToolErrorRate:             0.0,
		TotalTokens:               20000,
		SpecInstructionCompliance: 1.0,
	}
	// judge: "correctness" starts with "judge_unavailable" → excluded
	// Only "clarity" = 6.0 counts → judge = 6.0
	// code = 10.0
	// tokenEfficiency = clamp(0,1, 20000/20000) = 1.0
	// process = (0*0.4 + 1.0*0.4 + 1.0*0.2) * 10 = (0 + 0.4 + 0.2) * 10 = 6.0
	// spec = 10.0
	// composite = 10.0*0.3 + 6.0*0.3 + 6.0*0.2 + 10.0*0.2
	//           = 3.0 + 1.8 + 1.2 + 2.0 = 8.0
	got := computeComposite(grade)
	want := 8.0
	if math.Abs(got-want) > 1e-3 {
		t.Errorf("computeComposite with judge_unavailable excluded = %v, want %v", got, want)
	}
}

func TestComputeComposite_NoJudgeScores(t *testing.T) {
	grade := models.Grade{
		TestPassRate:              0.5,
		JudgeScores:               nil,
		RanValidation:             true,
		ToolErrorRate:             0.0,
		TotalTokens:               20000,
		SpecInstructionCompliance: 0.5,
	}
	// code = 5.0
	// judge = 0.0 (no scores)
	// tokenEfficiency = 1.0
	// process = (1*0.4 + 1.0*0.4 + 1.0*0.2) * 10 = 10.0
	// spec = 5.0
	// composite = 5.0*0.3 + 0.0*0.3 + 10.0*0.2 + 5.0*0.2
	//           = 1.5 + 0.0 + 2.0 + 1.0 = 4.5
	got := computeComposite(grade)
	want := 4.5
	if math.Abs(got-want) > 1e-3 {
		t.Errorf("computeComposite no judge = %v, want %v", got, want)
	}
}

func TestComputeComposite_RoundTripDeterminism(t *testing.T) {
	grade := models.Grade{
		TestPassRate:              0.75,
		JudgeScores:               map[string]float64{"q1": 5.5, "q2": 7.5},
		RanValidation:             true,
		ToolErrorRate:             0.25,
		TotalTokens:               30000,
		SpecInstructionCompliance: 0.6,
	}
	first := computeComposite(grade)
	for i := 0; i < 100; i++ {
		got := computeComposite(grade)
		if got != first {
			t.Errorf("computeComposite not deterministic: iteration %d got %v, first was %v", i, got, first)
		}
	}
}

func TestComputeComposite_AllJudgeUnavailable(t *testing.T) {
	grade := models.Grade{
		TestPassRate: 0.5,
		JudgeScores:  map[string]float64{"d1": 9.0},
		JudgeRationales: map[string]string{
			"d1": "judge_unavailable: provider timeout",
		},
		RanValidation:             false,
		ToolErrorRate:             0.0,
		TotalTokens:               20000,
		SpecInstructionCompliance: 0.5,
	}
	// All judge dims excluded → judge = 0.0
	// code = 5.0
	// tokenEfficiency = 1.0
	// process = (0*0.4 + 1.0*0.4 + 1.0*0.2) * 10 = 6.0
	// spec = 5.0
	// composite = 5.0*0.3 + 0.0*0.3 + 6.0*0.2 + 5.0*0.2
	//           = 1.5 + 0 + 1.2 + 1.0 = 3.7
	got := computeComposite(grade)
	want := 3.7
	if math.Abs(got-want) > 1e-3 {
		t.Errorf("computeComposite all judge_unavailable = %v, want %v", got, want)
	}
}

func TestComputeComposite_TokensAboveCap(t *testing.T) {
	grade := models.Grade{
		TestPassRate:              1.0,
		JudgeScores:               map[string]float64{"q": 10.0},
		RanValidation:             true,
		ToolErrorRate:             0.0,
		TotalTokens:               100000, // well above 20000 cap
		SpecInstructionCompliance: 1.0,
	}
	// tokenEfficiency = clamp(0,1, 20000/100000) = 0.2
	// process = (1*0.4 + 1.0*0.4 + 0.2*0.2) * 10 = (0.4 + 0.4 + 0.04) * 10 = 8.4
	// code = 10, judge = 10, spec = 10
	// composite = 10*0.3 + 10*0.3 + 8.4*0.2 + 10*0.2
	//           = 3 + 3 + 1.68 + 2 = 9.68
	got := computeComposite(grade)
	want := 9.68
	if math.Abs(got-want) > 1e-3 {
		t.Errorf("computeComposite tokens above cap = %v, want %v", got, want)
	}
}
