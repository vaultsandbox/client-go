// Package spamanalysis provides types and utilities for working with
// spam analysis results from Rspamd.
package spamanalysis

// SpamAction represents the recommended action from spam analysis.
type SpamAction string

const (
	// ActionNoAction indicates the email is clean and should be delivered normally.
	ActionNoAction SpamAction = "no action"
	// ActionGreylist indicates a temporary rejection to retry later (anti-spam technique).
	ActionGreylist SpamAction = "greylist"
	// ActionAddHeader indicates spam headers should be added but email delivered.
	ActionAddHeader SpamAction = "add header"
	// ActionRewriteSubject indicates the subject should be modified to indicate spam.
	ActionRewriteSubject SpamAction = "rewrite subject"
	// ActionSoftReject indicates a temporary rejection (4xx SMTP code).
	ActionSoftReject SpamAction = "soft reject"
	// ActionReject indicates a permanent rejection (5xx SMTP code).
	ActionReject SpamAction = "reject"
)

// SpamStatus represents the status of spam analysis.
type SpamStatus string

const (
	// StatusAnalyzed indicates the email was successfully analyzed.
	StatusAnalyzed SpamStatus = "analyzed"
	// StatusSkipped indicates analysis was skipped (disabled globally or per-inbox).
	StatusSkipped SpamStatus = "skipped"
	// StatusError indicates analysis failed (Rspamd unavailable, timeout, etc.).
	StatusError SpamStatus = "error"
)

// SpamSymbol represents an individual spam rule that was triggered during analysis.
type SpamSymbol struct {
	// Name is the rule identifier (e.g., "DKIM_SIGNED", "FORGED_SENDER").
	// Rspamd symbol names follow conventions:
	//   - Positive scores: spam indicators (e.g., "FORGED_SENDER")
	//   - Negative scores: ham indicators (e.g., "DKIM_SIGNED")
	Name string `json:"name"`
	// Score is the contribution from this rule.
	// Positive values increase spam score, negative values decrease it.
	Score float64 `json:"score"`
	// Description is a human-readable description of what this rule detects.
	Description string `json:"description,omitempty"`
	// Options contains additional context or matched values
	// (e.g., for URL rules, contains the matched URLs).
	Options []string `json:"options,omitempty"`
}

// SpamAnalysis contains the result of spam analysis for an email.
type SpamAnalysis struct {
	// Status indicates the analysis result status.
	//   - "analyzed": Successfully analyzed by Rspamd
	//   - "skipped": Analysis was skipped (disabled globally or per-inbox)
	//   - "error": Analysis failed (Rspamd unavailable, timeout, etc.)
	Status SpamStatus `json:"status"`
	// Score is the overall spam score (positive = more spammy).
	// Only present when Status is "analyzed".
	// Typical range: -10 to +15, but can vary.
	Score *float64 `json:"score,omitempty"`
	// RequiredScore is the threshold for spam classification.
	// Emails with Score >= RequiredScore are considered spam.
	// Default Rspamd threshold is typically 6.0.
	RequiredScore *float64 `json:"requiredScore,omitempty"`
	// Action is the recommended action from Rspamd based on score thresholds.
	Action SpamAction `json:"action,omitempty"`
	// IsSpam indicates whether the email is classified as spam.
	// True when Score >= RequiredScore.
	IsSpam *bool `json:"isSpam,omitempty"`
	// Symbols contains the list of triggered spam rules with their scores.
	Symbols []SpamSymbol `json:"symbols,omitempty"`
	// ProcessingTimeMs is the time taken for spam analysis in milliseconds.
	ProcessingTimeMs *int `json:"processingTimeMs,omitempty"`
	// Info contains additional information about the analysis.
	// Contains error messages when Status is "error".
	// Contains skip reason when Status is "skipped".
	Info string `json:"info,omitempty"`
}

// GetScore returns the spam score or nil if not analyzed.
// This is a convenience method that returns nil when analysis was not performed.
func (s *SpamAnalysis) GetScore() *float64 {
	if s == nil || s.Status != StatusAnalyzed {
		return nil
	}
	return s.Score
}

// GetIsSpam returns whether the email is spam, or nil if unknown.
// Returns nil if analysis was not performed or status is not "analyzed".
func (s *SpamAnalysis) GetIsSpam() *bool {
	if s == nil || s.Status != StatusAnalyzed {
		return nil
	}
	return s.IsSpam
}

// WasAnalyzed returns true if spam analysis was successfully performed.
func (s *SpamAnalysis) WasAnalyzed() bool {
	return s != nil && s.Status == StatusAnalyzed
}

// WasSkipped returns true if spam analysis was intentionally skipped.
func (s *SpamAnalysis) WasSkipped() bool {
	return s != nil && s.Status == StatusSkipped
}

// HasError returns true if spam analysis failed with an error.
func (s *SpamAnalysis) HasError() bool {
	return s != nil && s.Status == StatusError
}

// SpamValidation provides a summary of spam analysis validation.
type SpamValidation struct {
	// Available indicates whether spam analysis results are available.
	Available bool
	// IsSpam indicates whether the email is classified as spam.
	// Only meaningful when Available is true.
	IsSpam bool
	// Score is the spam score.
	// Only meaningful when Available is true.
	Score float64
	// Action is the recommended action.
	// Only meaningful when Available is true.
	Action SpamAction
	// Reason contains the skip reason or error message when Available is false.
	Reason string
}

// Validate validates the spam analysis results and provides a summary.
// It returns a SpamValidation struct with details about the analysis.
func (s *SpamAnalysis) Validate() SpamValidation {
	if s == nil {
		return SpamValidation{
			Available: false,
			Reason:    "no spam analysis results available",
		}
	}

	switch s.Status {
	case StatusAnalyzed:
		score := float64(0)
		if s.Score != nil {
			score = *s.Score
		}
		isSpam := false
		if s.IsSpam != nil {
			isSpam = *s.IsSpam
		}
		return SpamValidation{
			Available: true,
			IsSpam:    isSpam,
			Score:     score,
			Action:    s.Action,
		}
	case StatusSkipped:
		return SpamValidation{
			Available: false,
			Reason:    s.Info,
		}
	case StatusError:
		return SpamValidation{
			Available: false,
			Reason:    s.Info,
		}
	default:
		return SpamValidation{
			Available: false,
			Reason:    "unknown status",
		}
	}
}

// CategorizeSymbols groups symbols by their effect on spam score.
// Returns three slices: spam indicators (positive score), ham indicators
// (negative score), and informational (zero score).
func CategorizeSymbols(symbols []SpamSymbol) (positive, negative, neutral []SpamSymbol) {
	for _, s := range symbols {
		if s.Score > 0 {
			positive = append(positive, s)
		} else if s.Score < 0 {
			negative = append(negative, s)
		} else {
			neutral = append(neutral, s)
		}
	}
	return
}
