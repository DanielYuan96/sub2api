package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	defaultImageCapabilityModel = "gpt-image-2"
	imageCapabilityListPageSize = 10000
)

var defaultImageCapabilityModels = []string{
	"gpt-image-2",
	"gpt-image-1.5",
	"gpt-image-1",
}

// ImageCapabilityService returns the API keys a user can actually use for the
// OpenAI Images API. It is a UX/discovery helper; gateway handlers remain the
// authoritative permission and billing boundary for real requests.
type ImageCapabilityService struct {
	apiKeyService        *APIKeyService
	userRepo             UserRepository
	subscriptionService  *SubscriptionService
	openAIGatewayService *OpenAIGatewayService
}

func NewImageCapabilityService(
	apiKeyService *APIKeyService,
	userRepo UserRepository,
	subscriptionService *SubscriptionService,
	openAIGatewayService *OpenAIGatewayService,
) *ImageCapabilityService {
	return &ImageCapabilityService{
		apiKeyService:        apiKeyService,
		userRepo:             userRepo,
		subscriptionService:  subscriptionService,
		openAIGatewayService: openAIGatewayService,
	}
}

type ImageCapableKeysResponse struct {
	DefaultModel string                 `json:"default_model"`
	Keys         []ImageCapableKey      `json:"keys"`
	EmptyReason  *string                `json:"empty_reason"`
	Models       []ImageCapabilityModel `json:"models"`
}

type ImageCapabilityModel struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Sizes       []string `json:"sizes"`
}

type ImageCapableKey struct {
	ID           int64                  `json:"id"`
	Name         string                 `json:"name"`
	MaskedKey    string                 `json:"masked_key"`
	Group        ImageCapableGroup      `json:"group"`
	Models       []ImageCapableKeyModel `json:"models"`
	DefaultModel string                 `json:"default_model"`
	BillingMode  string                 `json:"billing_mode"`
	Limits       ImageCapableKeyLimits  `json:"limits"`
}

type ImageCapableGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type ImageCapableKeyModel struct {
	ID          string `json:"id"`
	MappedModel string `json:"mapped_model"`
	Capability  string `json:"capability"`
}

type ImageCapableKeyLimits struct {
	UserConcurrency   int `json:"user_concurrency"`
	EffectiveRPMLimit int `json:"effective_rpm_limit"`
}

func (s *ImageCapabilityService) ListUserImageCapableKeys(ctx context.Context, userID int64) (*ImageCapableKeysResponse, error) {
	resp := &ImageCapableKeysResponse{
		DefaultModel: defaultImageCapabilityModel,
		Keys:         []ImageCapableKey{},
		Models:       defaultImageCapabilityCatalog(),
	}
	if s == nil || s.apiKeyService == nil || s.userRepo == nil || s.openAIGatewayService == nil {
		reason := "IMAGE_CAPABILITY_SERVICE_UNAVAILABLE"
		resp.EmptyReason = &reason
		return resp, nil
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil || !user.IsActive() {
		reason := "USER_INACTIVE"
		resp.EmptyReason = &reason
		return resp, nil
	}

	keys, _, err := s.apiKeyService.List(ctx, userID, pagination.PaginationParams{
		Page:      1,
		PageSize:  imageCapabilityListPageSize,
		SortBy:    "created_at",
		SortOrder: "desc",
	}, APIKeyListFilters{})
	if err != nil {
		return nil, err
	}

	for i := range keys {
		capable, ok := s.buildImageCapableKey(ctx, user, &keys[i])
		if !ok {
			continue
		}
		resp.Keys = append(resp.Keys, capable)
	}

	sort.SliceStable(resp.Keys, func(i, j int) bool {
		if resp.Keys[i].Group.ID != resp.Keys[j].Group.ID {
			return resp.Keys[i].Group.ID < resp.Keys[j].Group.ID
		}
		return strings.ToLower(resp.Keys[i].Name) < strings.ToLower(resp.Keys[j].Name)
	})

	if len(resp.Keys) == 0 {
		reason := "NO_IMAGE_CAPABLE_KEYS"
		resp.EmptyReason = &reason
	}
	return resp, nil
}

func (s *ImageCapabilityService) buildImageCapableKey(ctx context.Context, user *User, key *APIKey) (ImageCapableKey, bool) {
	if key == nil || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() || keyRateLimitExhausted(key) {
		return ImageCapableKey{}, false
	}
	group := key.Group
	if group == nil || key.GroupID == nil || !group.IsActive() || group.Platform != PlatformOpenAI {
		return ImageCapableKey{}, false
	}

	billingMode, ok := s.checkImageCapabilityBilling(ctx, user, group)
	if !ok {
		return ImageCapableKey{}, false
	}

	models := make([]ImageCapableKeyModel, 0, len(defaultImageCapabilityModels))
	for _, model := range defaultImageCapabilityModels {
		capableModel, ok := s.buildImageCapableModel(ctx, group, model)
		if ok {
			models = append(models, capableModel)
		}
	}
	if len(models) == 0 {
		return ImageCapableKey{}, false
	}

	return ImageCapableKey{
		ID:        key.ID,
		Name:      key.Name,
		MaskedKey: maskAPIKeyForDisplay(key.Key),
		Group: ImageCapableGroup{
			ID:       group.ID,
			Name:     group.Name,
			Platform: group.Platform,
		},
		Models:       models,
		DefaultModel: chooseImageDefaultModel(models),
		BillingMode:  billingMode,
		Limits: ImageCapableKeyLimits{
			UserConcurrency:   user.Concurrency,
			EffectiveRPMLimit: effectiveRPMLimit(user, group),
		},
	}, true
}

func (s *ImageCapabilityService) checkImageCapabilityBilling(ctx context.Context, user *User, group *Group) (string, bool) {
	if group.IsSubscriptionType() {
		if s.subscriptionService == nil {
			return "subscription", false
		}
		sub, err := s.subscriptionService.GetActiveSubscription(ctx, user.ID, group.ID)
		if err != nil {
			return "subscription", false
		}
		if _, err := s.subscriptionService.ValidateAndCheckLimits(sub, group); err != nil {
			return "subscription", false
		}
		return "subscription", true
	}
	if user.Balance <= 0 {
		return "balance", false
	}
	return "balance", true
}

func (s *ImageCapabilityService) buildImageCapableModel(ctx context.Context, group *Group, model string) (ImageCapableKeyModel, bool) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-image-") {
		return ImageCapableKeyModel{}, false
	}
	groupID := group.ID
	mapping, _ := s.openAIGatewayService.ResolveChannelMappingAndRestrict(ctx, &groupID, model)
	mappedModel := strings.TrimSpace(mapping.MappedModel)
	if mappedModel == "" {
		mappedModel = model
	}

	selection, _, err := s.openAIGatewayService.SelectAccountWithSchedulerForImages(
		ctx,
		&groupID,
		"image-capability:"+strconv.FormatInt(groupID, 10)+":"+model,
		model,
		nil,
		OpenAIImagesCapabilityNative,
	)
	if selection != nil && selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
	if err != nil || selection == nil || selection.Account == nil {
		return ImageCapableKeyModel{}, false
	}

	return ImageCapableKeyModel{
		ID:          model,
		MappedModel: mappedModel,
		Capability:  string(OpenAIImagesCapabilityNative),
	}, true
}

func keyRateLimitExhausted(key *APIKey) bool {
	if key.RateLimit5h > 0 && key.EffectiveUsage5h() >= key.RateLimit5h {
		return true
	}
	if key.RateLimit1d > 0 && key.EffectiveUsage1d() >= key.RateLimit1d {
		return true
	}
	if key.RateLimit7d > 0 && key.EffectiveUsage7d() >= key.RateLimit7d {
		return true
	}
	return false
}

func effectiveRPMLimit(user *User, group *Group) int {
	if group != nil && group.RPMLimit > 0 {
		return group.RPMLimit
	}
	if user != nil && user.RPMLimit > 0 {
		return user.RPMLimit
	}
	return 0
}

func chooseImageDefaultModel(models []ImageCapableKeyModel) string {
	for _, model := range models {
		if model.ID == defaultImageCapabilityModel {
			return model.ID
		}
	}
	if len(models) == 0 {
		return ""
	}
	return models[0].ID
}

func maskAPIKeyForDisplay(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return "****"
	}
	return key[:8] + "..." + key[len(key)-4:]
}

func defaultImageCapabilityCatalog() []ImageCapabilityModel {
	sizes := []string{"auto", "1024x1024", "1536x1024", "1024x1536"}
	return []ImageCapabilityModel{
		{ID: "gpt-image-2", DisplayName: "GPT Image 2", Sizes: sizes},
		{ID: "gpt-image-1.5", DisplayName: "GPT Image 1.5", Sizes: sizes},
		{ID: "gpt-image-1", DisplayName: "GPT Image 1", Sizes: sizes},
	}
}
