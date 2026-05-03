package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type registrationEmailBlocklistRepository struct {
	db *sql.DB
}

func NewRegistrationEmailBlocklistRepository(db *sql.DB) service.RegistrationEmailBlocklistRepository {
	return &registrationEmailBlocklistRepository{db: db}
}

func (r *registrationEmailBlocklistRepository) IsRegistrationEmailBlocked(ctx context.Context, email string) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}

	normalizedEmail, domains, ok := service.RegistrationEmailBlocklistLookup(email)
	if !ok {
		return false, nil
	}

	args := []any{normalizedEmail}
	conditions := []string{"(match_type = 'email' AND pattern = $1)"}
	for _, domain := range domains {
		args = append(args, domain)
		conditions = append(conditions, fmt.Sprintf("(match_type = 'domain' AND pattern = $%d)", len(args)))
	}

	query := `
SELECT EXISTS (
    SELECT 1
    FROM registration_email_blocklist
    WHERE enabled = TRUE
      AND (` + strings.Join(conditions, " OR ") + `)
)`

	var blocked bool
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&blocked); err != nil {
		return false, err
	}
	return blocked, nil
}

func (r *registrationEmailBlocklistRepository) UpsertRegistrationEmailBlocklist(ctx context.Context, entries []service.RegistrationEmailBlocklistEntry) (int, error) {
	if r == nil || r.db == nil || len(entries) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO registration_email_blocklist (pattern, match_type, source, reason, enabled)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (match_type, pattern) DO UPDATE
SET source = CASE
        WHEN registration_email_blocklist.source = 'manual' THEN registration_email_blocklist.source
        ELSE EXCLUDED.source
    END,
    reason = CASE
        WHEN registration_email_blocklist.source = 'manual' THEN registration_email_blocklist.reason
        ELSE COALESCE(NULLIF(EXCLUDED.reason, ''), registration_email_blocklist.reason)
    END,
    enabled = CASE
        WHEN registration_email_blocklist.source = 'manual' THEN registration_email_blocklist.enabled
        ELSE EXCLUDED.enabled
    END,
    last_seen_at = NOW(),
    updated_at = NOW()`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()

	count := 0
	for _, entry := range entries {
		matchType := strings.ToLower(strings.TrimSpace(entry.MatchType))
		pattern, err := service.NormalizeRegistrationEmailBlocklistPattern(entry.Pattern, matchType)
		if err != nil || pattern == "" {
			continue
		}
		source := normalizeRegistrationEmailBlocklistSource(entry.Source)
		if _, err := stmt.ExecContext(ctx, pattern, matchType, source, strings.TrimSpace(entry.Reason), entry.Enabled); err != nil {
			return count, err
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return count, err
	}
	return count, nil
}

func normalizeRegistrationEmailBlocklistSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case service.RegistrationEmailBlocklistSourceSeed:
		return service.RegistrationEmailBlocklistSourceSeed
	case service.RegistrationEmailBlocklistSourceExternal:
		return service.RegistrationEmailBlocklistSourceExternal
	default:
		return service.RegistrationEmailBlocklistSourceManual
	}
}
