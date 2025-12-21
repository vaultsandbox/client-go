package authresults

// AuthResults contains all email authentication check results.
type AuthResults struct {
	SPF        *SPFResult        `json:"spf,omitempty"`
	DKIM       []DKIMResult      `json:"dkim,omitempty"`
	DMARC      *DMARCResult      `json:"dmarc,omitempty"`
	ReverseDNS *ReverseDNSResult `json:"reverseDns,omitempty"`
}

// SPFResult represents an SPF check result.
type SPFResult struct {
	Status string `json:"status"` // pass, fail, softfail, neutral, none, temperror, permerror
	Domain string `json:"domain,omitempty"`
	IP     string `json:"ip,omitempty"`
	Info   string `json:"info,omitempty"`
}

// DKIMResult represents a DKIM check result.
type DKIMResult struct {
	Status   string `json:"status"` // pass, fail, none
	Domain   string `json:"domain,omitempty"`
	Selector string `json:"selector,omitempty"`
	Info     string `json:"info,omitempty"`
}

// DMARCResult represents a DMARC check result.
type DMARCResult struct {
	Status  string `json:"status"` // pass, fail, none
	Policy  string `json:"policy,omitempty"` // none, quarantine, reject
	Aligned bool   `json:"aligned,omitempty"`
	Domain  string `json:"domain,omitempty"`
	Info    string `json:"info,omitempty"`
}

// ReverseDNSResult represents a reverse DNS check result.
type ReverseDNSResult struct {
	Status   string `json:"status"` // pass, fail, none
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Info     string `json:"info,omitempty"`
}

// AuthValidation provides a summary of email authentication validation.
type AuthValidation struct {
	// Passed indicates whether all primary checks (SPF, DKIM, DMARC) passed.
	Passed bool `json:"passed"`
	// SPFPassed indicates whether the SPF check passed.
	SPFPassed bool `json:"spfPassed"`
	// DKIMPassed indicates whether at least one DKIM signature passed.
	DKIMPassed bool `json:"dkimPassed"`
	// DMARCPassed indicates whether the DMARC check passed.
	DMARCPassed bool `json:"dmarcPassed"`
	// ReverseDNSPassed indicates whether the reverse DNS check passed.
	ReverseDNSPassed bool `json:"reverseDnsPassed"`
	// Failures contains descriptive messages for any failed checks.
	Failures []string `json:"failures"`
}

// Validate validates the authentication results and provides a summary.
// It returns an AuthValidation struct with details about each check.
func (a *AuthResults) Validate() AuthValidation {
	if a == nil {
		return AuthValidation{
			Passed:   false,
			Failures: []string{"no authentication results available"},
		}
	}

	var failures []string

	// Check SPF
	spfPassed := a.SPF != nil && a.SPF.Status == "pass"
	if a.SPF != nil && !spfPassed {
		msg := "SPF check failed: " + a.SPF.Status
		if a.SPF.Domain != "" {
			msg += " (domain: " + a.SPF.Domain + ")"
		}
		failures = append(failures, msg)
	}

	// Check DKIM (at least one signature must pass)
	dkimPassed := false
	if len(a.DKIM) > 0 {
		for _, dkim := range a.DKIM {
			if dkim.Status == "pass" {
				dkimPassed = true
				break
			}
		}
		if !dkimPassed {
			var failedDomains []string
			for _, dkim := range a.DKIM {
				if dkim.Status != "pass" && dkim.Domain != "" {
					failedDomains = append(failedDomains, dkim.Domain)
				}
			}
			msg := "DKIM signature failed"
			if len(failedDomains) > 0 {
				msg += ": " + joinStrings(failedDomains, ", ")
			}
			failures = append(failures, msg)
		}
	}

	// Check DMARC
	dmarcPassed := a.DMARC != nil && a.DMARC.Status == "pass"
	if a.DMARC != nil && !dmarcPassed {
		msg := "DMARC policy: " + a.DMARC.Status
		if a.DMARC.Policy != "" {
			msg += " (policy: " + a.DMARC.Policy + ")"
		}
		failures = append(failures, msg)
	}

	// Check Reverse DNS
	reverseDNSPassed := a.ReverseDNS != nil && a.ReverseDNS.Status == "pass"
	if a.ReverseDNS != nil && !reverseDNSPassed {
		msg := "Reverse DNS check failed: " + a.ReverseDNS.Status
		if a.ReverseDNS.Hostname != "" {
			msg += " (hostname: " + a.ReverseDNS.Hostname + ")"
		}
		failures = append(failures, msg)
	}

	// Ensure failures is never nil
	if failures == nil {
		failures = []string{}
	}

	return AuthValidation{
		Passed:           spfPassed && dkimPassed && dmarcPassed,
		SPFPassed:        spfPassed,
		DKIMPassed:       dkimPassed,
		DMARCPassed:      dmarcPassed,
		ReverseDNSPassed: reverseDNSPassed,
		Failures:         failures,
	}
}

// joinStrings joins strings with a separator (helper to avoid strings import).
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// IsPassing returns true if all primary authentication checks (SPF, DKIM, DMARC) passed.
// This is a convenience method equivalent to calling Validate().Passed.
// Note: Reverse DNS is not included in this check.
func (a *AuthResults) IsPassing() bool {
	return a.Validate().Passed
}
