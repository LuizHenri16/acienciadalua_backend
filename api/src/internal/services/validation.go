package services

import (
	"strings"
)

func isEmail(value string) bool {
	// Basic address validation: no whitespace, one "@" with non-empty local
	// part and domain, and a dot in the domain.
	if strings.ContainsAny(value, " \t\n\r") {
		return false
	}
	at := strings.LastIndex(value, "@")
	if at <= 0 || at == len(value)-1 {
		return false
	}
	domain := value[at+1:]
	dot := strings.Index(domain, ".")
	return dot > 0 && dot < len(domain)-1
}
