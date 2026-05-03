CREATE TABLE IF NOT EXISTS registration_email_blocklist (
    id           BIGSERIAL PRIMARY KEY,
    pattern      VARCHAR(320) NOT NULL,
    match_type   VARCHAR(20)  NOT NULL,
    source       VARCHAR(32)  NOT NULL DEFAULT 'manual',
    reason       TEXT         NOT NULL DEFAULT '',
    enabled      BOOLEAN      NOT NULL DEFAULT TRUE,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT registration_email_blocklist_match_type_check
        CHECK (match_type IN ('email', 'domain')),
    CONSTRAINT registration_email_blocklist_source_check
        CHECK (source IN ('manual', 'seed', 'external'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_registration_email_blocklist_type_pattern
    ON registration_email_blocklist (match_type, pattern);

CREATE INDEX IF NOT EXISTS idx_registration_email_blocklist_enabled_type
    ON registration_email_blocklist (enabled, match_type);

COMMENT ON TABLE registration_email_blocklist IS '注册邮箱黑名单，支持完整邮箱和域名后缀匹配';
COMMENT ON COLUMN registration_email_blocklist.pattern IS '规范化后的邮箱地址或域名，不带 @';
COMMENT ON COLUMN registration_email_blocklist.match_type IS 'email=完整邮箱匹配，domain=域名/父域名后缀匹配';
COMMENT ON COLUMN registration_email_blocklist.source IS 'manual=手动添加，seed=初始化内置，external=外部一次性邮箱列表同步';

INSERT INTO registration_email_blocklist (pattern, match_type, source, reason)
VALUES
    ('10minutemail.com', 'domain', 'seed', 'common disposable email domain'),
    ('10minutemail.net', 'domain', 'seed', 'common disposable email domain'),
    ('10minutemail.org', 'domain', 'seed', 'common disposable email domain'),
    ('1secmail.com', 'domain', 'seed', 'common disposable email domain'),
    ('1secmail.net', 'domain', 'seed', 'common disposable email domain'),
    ('1secmail.org', 'domain', 'seed', 'common disposable email domain'),
    ('br.cloudvxz.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('bwmyga.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('deltajohnsons.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('dispostable.com', 'domain', 'seed', 'common disposable email domain'),
    ('emailondeck.com', 'domain', 'seed', 'common disposable email domain'),
    ('esiix.com', 'domain', 'seed', 'common disposable email domain'),
    ('fakeinbox.com', 'domain', 'seed', 'common disposable email domain'),
    ('fakemail.net', 'domain', 'seed', 'common disposable email domain'),
    ('fakemailgenerator.com', 'domain', 'seed', 'common disposable email domain'),
    ('getnada.com', 'domain', 'seed', 'common disposable email domain'),
    ('gixpos.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('gongjua.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('grr.la', 'domain', 'seed', 'common disposable email domain'),
    ('guerrillamail.com', 'domain', 'seed', 'common disposable email domain'),
    ('guerrillamail.net', 'domain', 'seed', 'common disposable email domain'),
    ('guerrillamail.org', 'domain', 'seed', 'common disposable email domain'),
    ('guerrillamailblock.com', 'domain', 'seed', 'common disposable email domain'),
    ('inboxbear.com', 'domain', 'seed', 'common disposable email domain'),
    ('kynninc.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('lnovic.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('lohinja.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('maildrop.cc', 'domain', 'seed', 'common disposable email domain'),
    ('mailinator.com', 'domain', 'seed', 'common disposable email domain'),
    ('mediaeast.uk', 'domain', 'seed', 'observed disposable registration domain'),
    ('mediaholy.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('mintemail.com', 'domain', 'seed', 'common disposable email domain'),
    ('mohmal.com', 'domain', 'seed', 'common disposable email domain'),
    ('mohmal.in', 'domain', 'seed', 'common disposable email domain'),
    ('nada.ltd', 'domain', 'seed', 'common disposable email domain'),
    ('nimail.cn', 'domain', 'seed', 'observed disposable registration domain'),
    ('ozsaip.com', 'domain', 'seed', 'observed disposable registration domain'),
    ('sharklasers.com', 'domain', 'seed', 'common disposable email domain'),
    ('spam4.me', 'domain', 'seed', 'common disposable email domain'),
    ('spamgourmet.com', 'domain', 'seed', 'common disposable email domain'),
    ('temp-mail.org', 'domain', 'seed', 'common disposable email domain'),
    ('tempmail.com', 'domain', 'seed', 'common disposable email domain'),
    ('tempmail.io', 'domain', 'seed', 'common disposable email domain'),
    ('tempmail.net', 'domain', 'seed', 'common disposable email domain'),
    ('throwawaymail.com', 'domain', 'seed', 'common disposable email domain'),
    ('trashmail.com', 'domain', 'seed', 'common disposable email domain'),
    ('trashmail.me', 'domain', 'seed', 'common disposable email domain'),
    ('trashmail.net', 'domain', 'seed', 'common disposable email domain'),
    ('vtmpj.net', 'domain', 'seed', 'observed disposable registration domain'),
    ('wwjmp.com', 'domain', 'seed', 'common disposable email domain'),
    ('xojxe.com', 'domain', 'seed', 'common disposable email domain'),
    ('yopmail.com', 'domain', 'seed', 'common disposable email domain'),
    ('yopmail.fr', 'domain', 'seed', 'common disposable email domain'),
    ('yopmail.net', 'domain', 'seed', 'common disposable email domain'),
    ('yzcalo.com', 'domain', 'seed', 'observed disposable registration domain')
ON CONFLICT (match_type, pattern) DO UPDATE
SET enabled = TRUE,
    last_seen_at = NOW(),
    updated_at = NOW();
