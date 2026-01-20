package spamanalysis

import (
	"encoding/json"
	"testing"
)

func TestSpamAnalysis_GetScore(t *testing.T) {
	tests := []struct {
		name     string
		analysis *SpamAnalysis
		want     *float64
	}{
		{
			name:     "nil analysis",
			analysis: nil,
			want:     nil,
		},
		{
			name:     "skipped status",
			analysis: &SpamAnalysis{Status: StatusSkipped},
			want:     nil,
		},
		{
			name:     "error status",
			analysis: &SpamAnalysis{Status: StatusError},
			want:     nil,
		},
		{
			name: "analyzed with score",
			analysis: &SpamAnalysis{
				Status: StatusAnalyzed,
				Score:  floatPtr(5.5),
			},
			want: floatPtr(5.5),
		},
		{
			name: "analyzed without score",
			analysis: &SpamAnalysis{
				Status: StatusAnalyzed,
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.analysis.GetScore()
			if !floatPtrEqual(got, tt.want) {
				t.Errorf("GetScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpamAnalysis_GetIsSpam(t *testing.T) {
	tests := []struct {
		name     string
		analysis *SpamAnalysis
		want     *bool
	}{
		{
			name:     "nil analysis",
			analysis: nil,
			want:     nil,
		},
		{
			name:     "skipped status",
			analysis: &SpamAnalysis{Status: StatusSkipped},
			want:     nil,
		},
		{
			name: "analyzed is spam",
			analysis: &SpamAnalysis{
				Status: StatusAnalyzed,
				IsSpam: boolPtr(true),
			},
			want: boolPtr(true),
		},
		{
			name: "analyzed not spam",
			analysis: &SpamAnalysis{
				Status: StatusAnalyzed,
				IsSpam: boolPtr(false),
			},
			want: boolPtr(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.analysis.GetIsSpam()
			if !boolPtrEqual(got, tt.want) {
				t.Errorf("GetIsSpam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpamAnalysis_StatusMethods(t *testing.T) {
	tests := []struct {
		name        string
		analysis    *SpamAnalysis
		wasAnalyzed bool
		wasSkipped  bool
		hasError    bool
	}{
		{
			name:        "nil analysis",
			analysis:    nil,
			wasAnalyzed: false,
			wasSkipped:  false,
			hasError:    false,
		},
		{
			name:        "analyzed status",
			analysis:    &SpamAnalysis{Status: StatusAnalyzed},
			wasAnalyzed: true,
			wasSkipped:  false,
			hasError:    false,
		},
		{
			name:        "skipped status",
			analysis:    &SpamAnalysis{Status: StatusSkipped},
			wasAnalyzed: false,
			wasSkipped:  true,
			hasError:    false,
		},
		{
			name:        "error status",
			analysis:    &SpamAnalysis{Status: StatusError},
			wasAnalyzed: false,
			wasSkipped:  false,
			hasError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.analysis.WasAnalyzed(); got != tt.wasAnalyzed {
				t.Errorf("WasAnalyzed() = %v, want %v", got, tt.wasAnalyzed)
			}
			if got := tt.analysis.WasSkipped(); got != tt.wasSkipped {
				t.Errorf("WasSkipped() = %v, want %v", got, tt.wasSkipped)
			}
			if got := tt.analysis.HasError(); got != tt.hasError {
				t.Errorf("HasError() = %v, want %v", got, tt.hasError)
			}
		})
	}
}

func TestSpamAnalysis_Validate(t *testing.T) {
	tests := []struct {
		name      string
		analysis  *SpamAnalysis
		available bool
		isSpam    bool
		score     float64
		action    SpamAction
		reason    string
	}{
		{
			name:      "nil analysis",
			analysis:  nil,
			available: false,
			reason:    "no spam analysis results available",
		},
		{
			name:      "skipped",
			analysis:  &SpamAnalysis{Status: StatusSkipped, Info: "disabled"},
			available: false,
			reason:    "disabled",
		},
		{
			name:      "error",
			analysis:  &SpamAnalysis{Status: StatusError, Info: "timeout"},
			available: false,
			reason:    "timeout",
		},
		{
			name: "analyzed clean",
			analysis: &SpamAnalysis{
				Status: StatusAnalyzed,
				Score:  floatPtr(1.5),
				IsSpam: boolPtr(false),
				Action: ActionNoAction,
			},
			available: true,
			isSpam:    false,
			score:     1.5,
			action:    ActionNoAction,
		},
		{
			name: "analyzed spam",
			analysis: &SpamAnalysis{
				Status: StatusAnalyzed,
				Score:  floatPtr(12.0),
				IsSpam: boolPtr(true),
				Action: ActionReject,
			},
			available: true,
			isSpam:    true,
			score:     12.0,
			action:    ActionReject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.analysis.Validate()
			if v.Available != tt.available {
				t.Errorf("Validate().Available = %v, want %v", v.Available, tt.available)
			}
			if v.IsSpam != tt.isSpam {
				t.Errorf("Validate().IsSpam = %v, want %v", v.IsSpam, tt.isSpam)
			}
			if v.Score != tt.score {
				t.Errorf("Validate().Score = %v, want %v", v.Score, tt.score)
			}
			if v.Action != tt.action {
				t.Errorf("Validate().Action = %v, want %v", v.Action, tt.action)
			}
			if v.Reason != tt.reason {
				t.Errorf("Validate().Reason = %v, want %v", v.Reason, tt.reason)
			}
		})
	}
}

func TestCategorizeSymbols(t *testing.T) {
	symbols := []SpamSymbol{
		{Name: "DKIM_SIGNED", Score: -0.1},
		{Name: "SPF_ALLOW", Score: -0.2},
		{Name: "FORGED_SENDER", Score: 3.0},
		{Name: "MISSING_HEADERS", Score: 2.0},
		{Name: "INFO_ONLY", Score: 0.0},
	}

	positive, negative, neutral := CategorizeSymbols(symbols)

	if len(positive) != 2 {
		t.Errorf("positive count = %d, want 2", len(positive))
	}
	if len(negative) != 2 {
		t.Errorf("negative count = %d, want 2", len(negative))
	}
	if len(neutral) != 1 {
		t.Errorf("neutral count = %d, want 1", len(neutral))
	}

	// Check positive symbols
	if positive[0].Name != "FORGED_SENDER" && positive[1].Name != "FORGED_SENDER" {
		t.Error("FORGED_SENDER not in positive")
	}

	// Check negative symbols
	found := false
	for _, s := range negative {
		if s.Name == "DKIM_SIGNED" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DKIM_SIGNED not in negative")
	}

	// Check neutral
	if neutral[0].Name != "INFO_ONLY" {
		t.Errorf("neutral[0].Name = %v, want INFO_ONLY", neutral[0].Name)
	}
}

func TestSpamAnalysis_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"status": "analyzed",
		"score": 5.5,
		"requiredScore": 6.0,
		"action": "add header",
		"isSpam": false,
		"symbols": [
			{"name": "DKIM_SIGNED", "score": -0.1, "description": "Valid DKIM signature"},
			{"name": "MISSING_MID", "score": 2.5, "description": "Missing Message-ID"}
		],
		"processingTimeMs": 45
	}`

	var analysis SpamAnalysis
	if err := json.Unmarshal([]byte(jsonData), &analysis); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if analysis.Status != StatusAnalyzed {
		t.Errorf("Status = %v, want %v", analysis.Status, StatusAnalyzed)
	}
	if analysis.Score == nil || *analysis.Score != 5.5 {
		t.Errorf("Score = %v, want 5.5", analysis.Score)
	}
	if analysis.RequiredScore == nil || *analysis.RequiredScore != 6.0 {
		t.Errorf("RequiredScore = %v, want 6.0", analysis.RequiredScore)
	}
	if analysis.Action != ActionAddHeader {
		t.Errorf("Action = %v, want %v", analysis.Action, ActionAddHeader)
	}
	if analysis.IsSpam == nil || *analysis.IsSpam != false {
		t.Errorf("IsSpam = %v, want false", analysis.IsSpam)
	}
	if len(analysis.Symbols) != 2 {
		t.Errorf("Symbols count = %d, want 2", len(analysis.Symbols))
	}
	if analysis.ProcessingTimeMs == nil || *analysis.ProcessingTimeMs != 45 {
		t.Errorf("ProcessingTimeMs = %v, want 45", analysis.ProcessingTimeMs)
	}
}

func TestSpamAnalysis_JSONUnmarshal_Skipped(t *testing.T) {
	jsonData := `{
		"status": "skipped",
		"info": "Spam analysis disabled"
	}`

	var analysis SpamAnalysis
	if err := json.Unmarshal([]byte(jsonData), &analysis); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if analysis.Status != StatusSkipped {
		t.Errorf("Status = %v, want %v", analysis.Status, StatusSkipped)
	}
	if analysis.Info != "Spam analysis disabled" {
		t.Errorf("Info = %v, want 'Spam analysis disabled'", analysis.Info)
	}
}

func TestSpamAnalysis_JSONUnmarshal_Error(t *testing.T) {
	jsonData := `{
		"status": "error",
		"processingTimeMs": 5001,
		"info": "Rspamd request timed out after 5000ms"
	}`

	var analysis SpamAnalysis
	if err := json.Unmarshal([]byte(jsonData), &analysis); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if analysis.Status != StatusError {
		t.Errorf("Status = %v, want %v", analysis.Status, StatusError)
	}
	if analysis.ProcessingTimeMs == nil || *analysis.ProcessingTimeMs != 5001 {
		t.Errorf("ProcessingTimeMs = %v, want 5001", analysis.ProcessingTimeMs)
	}
	if analysis.Info != "Rspamd request timed out after 5000ms" {
		t.Errorf("Info = %v, want 'Rspamd request timed out after 5000ms'", analysis.Info)
	}
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}

func floatPtrEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
