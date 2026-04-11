package errmapping

// DomainRuleName returns the Name of the first matching rule in DomainRules(), or "" if none.
// Use for metrics aligned with errors.Is(..., apierrors.*) and HTTP/gRPC Problem mapping.
func DomainRuleName(err error) string {
	if err == nil {
		return ""
	}
	rules := DomainRules()
	for i := range rules {
		if rules[i].Match(err) {
			return rules[i].Name
		}
	}
	return ""
}

// ContractPipelineRuleName returns the Name of the first matching rule in ContractPipelineRules(), or "" for the default pipeline failure case.
func ContractPipelineRuleName(err error) string {
	if err == nil {
		return ""
	}
	rules := ContractPipelineRules()
	for i := range rules {
		if rules[i].Match(err) {
			return rules[i].Name
		}
	}
	return ""
}
