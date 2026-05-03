INSERT INTO settings (key, value)
VALUES
    ('disposable_email_sync_enabled', 'true'),
    ('disposable_email_sync_url', 'https://disposable.github.io/disposable-email-domains/domains.txt'),
    ('disposable_email_sync_interval_hours', '24')
ON CONFLICT (key) DO NOTHING;
