package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var registrationEmailDomainPattern = regexp.MustCompile(
	`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`,
)

// RegistrationEmailSuffix extracts normalized suffix in "@domain" form.
func RegistrationEmailSuffix(email string) string {
	_, domain, ok := splitEmailForPolicy(email)
	if !ok {
		return ""
	}
	return "@" + domain
}

// IsRegistrationEmailSuffixAllowed checks whether an email is allowed by suffix whitelist.
// Empty whitelist means allow all.
func IsRegistrationEmailSuffixAllowed(email string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true
	}
	suffix := RegistrationEmailSuffix(email)
	if suffix == "" {
		return false
	}
	for _, allowed := range whitelist {
		if suffix == allowed {
			return true
		}
	}
	return false
}

// RegistrationEmailBlocklistLookup normalizes an email and returns the exact
// email key plus domain suffix candidates for blacklist lookup.
func RegistrationEmailBlocklistLookup(raw string) (email string, domains []string, ok bool) {
	local, domain, ok := splitEmailForPolicy(raw)
	if !ok {
		return "", nil, false
	}
	return local + "@" + domain, registrationEmailDomainCandidates(domain), true
}

func NormalizeRegistrationEmailBlocklistPattern(raw, matchType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(matchType)) {
	case "email":
		local, domain, ok := splitEmailForPolicy(raw)
		if !ok {
			return "", fmt.Errorf("invalid email blocklist pattern: %q", raw)
		}
		return local + "@" + domain, nil
	case "domain":
		return normalizeRegistrationEmailBlocklistDomain(raw)
	default:
		return "", fmt.Errorf("invalid email blocklist match type: %q", matchType)
	}
}

func ParseDisposableEmailDomainList(raw string) []string {
	lines := strings.Split(raw, "\n")
	seen := make(map[string]struct{}, len(lines))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		domain, err := normalizeRegistrationEmailBlocklistDomain(line)
		if err != nil || domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		out = append(out, domain)
	}
	return out
}

// NormalizeRegistrationEmailSuffixWhitelist normalizes and validates suffix whitelist items.
func NormalizeRegistrationEmailSuffixWhitelist(raw []string) ([]string, error) {
	return normalizeRegistrationEmailSuffixWhitelist(raw, true)
}

// ParseRegistrationEmailSuffixWhitelist parses persisted JSON into normalized suffixes.
// Invalid entries are ignored to keep old misconfigurations from breaking runtime reads.
func ParseRegistrationEmailSuffixWhitelist(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []string{}
	}
	normalized, _ := normalizeRegistrationEmailSuffixWhitelist(items, false)
	if len(normalized) == 0 {
		return []string{}
	}
	return normalized
}

func normalizeRegistrationEmailSuffixWhitelist(raw []string, strict bool) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		normalized, err := normalizeRegistrationEmailSuffix(item)
		if err != nil {
			if strict {
				return nil, err
			}
			continue
		}
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func normalizeRegistrationEmailSuffix(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}

	domain := value
	if strings.Contains(value, "@") {
		if !strings.HasPrefix(value, "@") || strings.Count(value, "@") != 1 {
			return "", fmt.Errorf("invalid email suffix: %q", raw)
		}
		domain = strings.TrimPrefix(value, "@")
	}

	if domain == "" || strings.Contains(domain, "@") || !registrationEmailDomainPattern.MatchString(domain) {
		return "", fmt.Errorf("invalid email suffix: %q", raw)
	}

	return "@" + domain, nil
}

func splitEmailForPolicy(raw string) (local string, domain string, ok bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || domain == "" || strings.Contains(domain, "@") {
		return "", "", false
	}
	return local, domain, true
}

func normalizeRegistrationEmailBlocklistDomain(raw string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(raw))
	domain = strings.TrimPrefix(domain, "@")
	if domain == "" {
		return "", nil
	}
	if strings.Contains(domain, "@") || !registrationEmailDomainPattern.MatchString(domain) {
		return "", fmt.Errorf("invalid email blocklist domain: %q", raw)
	}
	return domain, nil
}

func registrationEmailDomainCandidates(domain string) []string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil
	}
	candidates := make([]string, 0, strings.Count(domain, ".")+1)
	for {
		candidates = append(candidates, domain)
		dot := strings.IndexByte(domain, '.')
		if dot < 0 {
			return candidates
		}
		domain = domain[dot+1:]
	}
}
