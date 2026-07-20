package sanitizer

import (
	"html"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/text/unicode/norm"
)

type Sanitizer struct {
	policy *bluemonday.Policy
}

func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		policy: bluemonday.StrictPolicy(),
	}
}

func (s *Sanitizer) Sanitize(input string) string {
	sanitized := html.UnescapeString(input)
	sanitized = s.policy.Sanitize(sanitized)
	sanitized = html.UnescapeString(sanitized)
	sanitized = strings.TrimSpace(sanitized)
	sanitized = norm.NFC.String(sanitized)
	sanitized = collapseSpaces(sanitized)
	sanitized = strings.ReplaceAll(sanitized, "\x00", "")
	return sanitized
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
