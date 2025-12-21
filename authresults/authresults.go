package authresults

// AuthResults contains all email authentication results.
type AuthResults struct {
	SPF        *SPFResult        `json:"spf,omitempty"`
	DKIM       *DKIMResult       `json:"dkim,omitempty"`
	DMARC      *DMARCResult      `json:"dmarc,omitempty"`
	ReverseDNS *ReverseDNSResult `json:"reverse_dns,omitempty"`
}

// SPFResult represents an SPF check result.
type SPFResult struct {
	Result string `json:"result"` // pass, fail, softfail, neutral, none, temperror, permerror
	Domain string `json:"domain"`
}

// DKIMResult represents a DKIM check result.
type DKIMResult struct {
	Result   string `json:"result"` // pass, fail, none, temperror, permerror
	Domain   string `json:"domain"`
	Selector string `json:"selector,omitempty"`
}

// DMARCResult represents a DMARC check result.
type DMARCResult struct {
	Result string `json:"result"` // pass, fail, none, temperror, permerror
	Domain string `json:"domain"`
	Policy string `json:"policy,omitempty"` // none, quarantine, reject
}

// ReverseDNSResult represents a reverse DNS lookup result.
type ReverseDNSResult struct {
	Hostname string `json:"hostname,omitempty"`
	Verified bool   `json:"verified"`
}

// IsPassing returns true if all authentication checks passed.
func (a *AuthResults) IsPassing() bool {
	if a == nil {
		return false
	}

	if a.SPF != nil && a.SPF.Result != "pass" {
		return false
	}

	if a.DKIM != nil && a.DKIM.Result != "pass" {
		return false
	}

	if a.DMARC != nil && a.DMARC.Result != "pass" {
		return false
	}

	return true
}
