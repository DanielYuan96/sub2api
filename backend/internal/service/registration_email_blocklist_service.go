package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

const (
	RegistrationEmailBlocklistMatchEmail  = "email"
	RegistrationEmailBlocklistMatchDomain = "domain"

	RegistrationEmailBlocklistSourceManual   = "manual"
	RegistrationEmailBlocklistSourceSeed     = "seed"
	RegistrationEmailBlocklistSourceExternal = "external"

	defaultDisposableEmailDomainSyncURL           = "https://disposable.github.io/disposable-email-domains/domains.txt"
	defaultDisposableEmailDomainSyncIntervalHours = 24
)

type RegistrationEmailBlocklistEntry struct {
	Pattern   string
	MatchType string
	Source    string
	Reason    string
	Enabled   bool
}

type RegistrationEmailBlocklistRepository interface {
	IsRegistrationEmailBlocked(ctx context.Context, email string) (bool, error)
	UpsertRegistrationEmailBlocklist(ctx context.Context, entries []RegistrationEmailBlocklistEntry) (int, error)
}

type DisposableEmailDomainClient interface {
	FetchDisposableEmailDomains(ctx context.Context, url string) ([]string, error)
}

type DisposableEmailDomainSyncService struct {
	repo           RegistrationEmailBlocklistRepository
	settingService *SettingService
	client         DisposableEmailDomainClient

	stopCh chan struct{}
	doneCh chan struct{}
	once   sync.Once
}

func NewDisposableEmailDomainHTTPClient(cfg *config.Config) DisposableEmailDomainClient {
	proxyURL := ""
	allowDirectOnProxyError := false
	if cfg != nil {
		proxyURL = cfg.Update.ProxyURL
		allowDirectOnProxyError = cfg.Security.ProxyFallback.AllowDirectOnError
	}
	client, err := httpclient.GetClient(httpclient.Options{
		Timeout:  30 * time.Second,
		ProxyURL: proxyURL,
	})
	if err != nil {
		if strings.TrimSpace(proxyURL) != "" && !allowDirectOnProxyError {
			return &disposableEmailDomainHTTPClient{initErr: fmt.Errorf("proxy client init failed and direct fallback is disabled: %w", err)}
		}
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &disposableEmailDomainHTTPClient{client: client}
}

func NewDisposableEmailDomainSyncService(
	repo RegistrationEmailBlocklistRepository,
	settingService *SettingService,
	client DisposableEmailDomainClient,
) *DisposableEmailDomainSyncService {
	return &DisposableEmailDomainSyncService{
		repo:           repo,
		settingService: settingService,
		client:         client,
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

func ProvideDisposableEmailDomainSyncService(
	repo RegistrationEmailBlocklistRepository,
	settingService *SettingService,
	client DisposableEmailDomainClient,
) *DisposableEmailDomainSyncService {
	svc := NewDisposableEmailDomainSyncService(repo, settingService, client)
	svc.Start()
	return svc
}

func (s *DisposableEmailDomainSyncService) Start() {
	if s == nil || s.repo == nil || s.client == nil {
		return
	}
	go s.loop()
}

func (s *DisposableEmailDomainSyncService) Stop() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		close(s.stopCh)
		select {
		case <-s.doneCh:
		case <-time.After(5 * time.Second):
			slog.Warn("disposable email domain sync stop timed out")
		}
	})
}

func (s *DisposableEmailDomainSyncService) SyncNow(ctx context.Context) error {
	if s == nil || s.repo == nil || s.client == nil {
		return nil
	}
	if s.settingService != nil && !s.settingService.IsDisposableEmailDomainSyncEnabled(ctx) {
		return nil
	}
	url := defaultDisposableEmailDomainSyncURL
	if s.settingService != nil {
		if configured := strings.TrimSpace(s.settingService.GetDisposableEmailDomainSyncURL(ctx)); configured != "" {
			url = configured
		}
	}

	domains, err := s.client.FetchDisposableEmailDomains(ctx, url)
	if err != nil {
		return err
	}
	entries := make([]RegistrationEmailBlocklistEntry, 0, len(domains))
	for _, domain := range domains {
		entries = append(entries, RegistrationEmailBlocklistEntry{
			Pattern:   domain,
			MatchType: RegistrationEmailBlocklistMatchDomain,
			Source:    RegistrationEmailBlocklistSourceExternal,
			Reason:    "external disposable email domain list",
			Enabled:   true,
		})
	}
	count, err := s.repo.UpsertRegistrationEmailBlocklist(ctx, entries)
	if err != nil {
		return err
	}
	slog.Info("disposable email domain sync completed", "domain_count", count, "url", url)
	return nil
}

func (s *DisposableEmailDomainSyncService) loop() {
	defer close(s.doneCh)

	s.runOnceWithTimeout()
	timer := time.NewTimer(s.interval())
	defer timer.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-timer.C:
			s.runOnceWithTimeout()
			timer.Reset(s.interval())
		}
	}
}

func (s *DisposableEmailDomainSyncService) runOnceWithTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := s.SyncNow(ctx); err != nil {
		slog.Warn("disposable email domain sync failed", "error", err)
	}
}

func (s *DisposableEmailDomainSyncService) interval() time.Duration {
	hours := defaultDisposableEmailDomainSyncIntervalHours
	if s != nil && s.settingService != nil {
		hours = s.settingService.GetDisposableEmailDomainSyncIntervalHours(context.Background())
	}
	if hours <= 0 {
		hours = defaultDisposableEmailDomainSyncIntervalHours
	}
	return time.Duration(hours) * time.Hour
}

type disposableEmailDomainHTTPClient struct {
	client  *http.Client
	initErr error
}

func (c *disposableEmailDomainHTTPClient) FetchDisposableEmailDomains(ctx context.Context, url string) ([]string, error) {
	if c.initErr != nil {
		return nil, c.initErr
	}
	if c.client == nil {
		c.client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(url), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch disposable email domains: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return ParseDisposableEmailDomainList(string(body)), nil
}
