//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRegistrationEmailSuffixWhitelist(t *testing.T) {
	got, err := NormalizeRegistrationEmailSuffixWhitelist([]string{"example.com", "@EXAMPLE.COM", " @foo.bar "})
	require.NoError(t, err)
	require.Equal(t, []string{"@example.com", "@foo.bar"}, got)
}

func TestNormalizeRegistrationEmailSuffixWhitelist_Invalid(t *testing.T) {
	_, err := NormalizeRegistrationEmailSuffixWhitelist([]string{"@invalid_domain"})
	require.Error(t, err)
}

func TestParseRegistrationEmailSuffixWhitelist(t *testing.T) {
	got := ParseRegistrationEmailSuffixWhitelist(`["example.com","@foo.bar","@invalid_domain"]`)
	require.Equal(t, []string{"@example.com", "@foo.bar"}, got)
}

func TestIsRegistrationEmailSuffixAllowed(t *testing.T) {
	require.True(t, IsRegistrationEmailSuffixAllowed("user@example.com", []string{"@example.com"}))
	require.False(t, IsRegistrationEmailSuffixAllowed("user@sub.example.com", []string{"@example.com"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("user@any.com", []string{}))
}

func TestRegistrationEmailBlocklistLookup(t *testing.T) {
	email, domains, ok := RegistrationEmailBlocklistLookup("User@Sub.Example.COM")
	require.True(t, ok)
	require.Equal(t, "user@sub.example.com", email)
	require.Equal(t, []string{"sub.example.com", "example.com", "com"}, domains)
}

func TestNormalizeRegistrationEmailBlocklistPattern(t *testing.T) {
	email, err := NormalizeRegistrationEmailBlocklistPattern("User@Example.COM", RegistrationEmailBlocklistMatchEmail)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", email)

	domain, err := NormalizeRegistrationEmailBlocklistPattern("@Example.COM", RegistrationEmailBlocklistMatchDomain)
	require.NoError(t, err)
	require.Equal(t, "example.com", domain)
}

func TestParseDisposableEmailDomainList(t *testing.T) {
	got := ParseDisposableEmailDomainList("mailinator.com\n# comment\n @Example.COM \ninvalid_domain\nmailinator.com")
	require.Equal(t, []string{"mailinator.com", "example.com"}, got)
}
