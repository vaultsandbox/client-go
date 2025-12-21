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

// IsPassing returns true if all authentication checks passed.
func (a *AuthResults) IsPassing() bool {
	if a == nil {
		return false
	}

	if a.SPF != nil && a.SPF.Status != "pass" {
		return false
	}

	// At least one DKIM must pass
	if len(a.DKIM) > 0 {
		dkimPassed := false
		for _, dkim := range a.DKIM {
			if dkim.Status == "pass" {
				dkimPassed = true
				break
			}
		}
		if !dkimPassed {
			return false
		}
	}

	if a.DMARC != nil && a.DMARC.Status != "pass" {
		return false
	}

	if a.ReverseDNS != nil && a.ReverseDNS.Status != "pass" {
		return false
	}

	return true
}
