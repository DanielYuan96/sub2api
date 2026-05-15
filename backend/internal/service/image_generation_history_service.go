package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type ImageGenerationStatus string

const (
	ImageGenerationStatusProcessing ImageGenerationStatus = "processing"
	ImageGenerationStatusCompleted  ImageGenerationStatus = "completed"
	ImageGenerationStatusFailed     ImageGenerationStatus = "failed"
	ImageGenerationStatusCancelled  ImageGenerationStatus = "cancelled"
)

var ErrImageGenerationAlreadyProcessing = errors.New("current image generation is still processing")
var errImageGenerationNotProcessing = errors.New("image generation task is not processing")

const (
	maxImageGenerationReferenceImages      = 3
	maxImageGenerationReferenceImageBytes  = 5 << 20
	maxImageGenerationReferenceImagesBytes = 15 << 20
)

type ImageGenerationRecord struct {
	ID              int64           `json:"id"`
	UserID          int64           `json:"user_id"`
	APIKeyID        *int64          `json:"api_key_id,omitempty"`
	APIKeyName      string          `json:"api_key_name"`
	Model           string          `json:"model"`
	Size            string          `json:"size"`
	Prompt          string          `json:"prompt"`
	Status          string          `json:"status"`
	Images          json.RawMessage `json:"images"`
	ReferenceImages json.RawMessage `json:"reference_images"`
	ErrorMessage    string          `json:"error_message"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
}

type CreateImageGenerationInput struct {
	UserID          int64
	APIKeyID        int64
	Model           string
	Size            string
	Prompt          string
	ReferenceImages json.RawMessage
}

type UpdateImageGenerationInput struct {
	UserID       int64
	ID           int64
	Status       string
	Images       json.RawMessage
	ErrorMessage string
}

type ImageGenerationHistoryService struct {
	db                  *sql.DB
	httpClient          *http.Client
	localGenerationsURL string
	localEditsURL       string
	activeMu            sync.Mutex
	activeTasks         map[int64]context.CancelFunc
}

func NewImageGenerationHistoryService(db *sql.DB, cfg *config.Config) *ImageGenerationHistoryService {
	port := 8080
	if cfg != nil && cfg.Server.Port > 0 {
		port = cfg.Server.Port
	}
	return &ImageGenerationHistoryService{
		db:                  db,
		httpClient:          &http.Client{Timeout: 10 * time.Minute},
		localGenerationsURL: fmt.Sprintf("http://127.0.0.1:%d/v1/images/generations", port),
		localEditsURL:       fmt.Sprintf("http://127.0.0.1:%d/v1/images/edits", port),
		activeTasks:         make(map[int64]context.CancelFunc),
	}
}

func (s *ImageGenerationHistoryService) List(ctx context.Context, userID int64, limit int) ([]ImageGenerationRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, api_key_id, api_key_name, model, size, prompt, status, images,
		       reference_images,
		       error_message, created_at, updated_at, completed_at
		FROM user_image_generations
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list image generations: %w", err)
	}
	defer rows.Close()

	records := make([]ImageGenerationRecord, 0)
	for rows.Next() {
		record, err := scanImageGenerationRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate image generations: %w", err)
	}
	return records, nil
}

func (s *ImageGenerationHistoryService) Create(ctx context.Context, input CreateImageGenerationInput) (*ImageGenerationRecord, error) {
	model := strings.TrimSpace(input.Model)
	size := strings.TrimSpace(input.Size)
	prompt := strings.TrimSpace(input.Prompt)
	if input.UserID <= 0 {
		return nil, errors.New("user_id is required")
	}
	if input.APIKeyID <= 0 {
		return nil, errors.New("api_key_id is required")
	}
	if model == "" {
		return nil, errors.New("model is required")
	}
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	referenceImages, err := normalizeImageGenerationReferenceImages(input.ReferenceImages)
	if err != nil {
		return nil, err
	}

	var hasProcessing bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_image_generations
			WHERE user_id = $1 AND status = 'processing'
		)
	`, input.UserID).Scan(&hasProcessing); err != nil {
		return nil, fmt.Errorf("check processing image generation: %w", err)
	}
	if hasProcessing {
		return nil, ErrImageGenerationAlreadyProcessing
	}

	var keyName string
	err = s.db.QueryRowContext(ctx, `
		SELECT name
		FROM api_keys
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, input.APIKeyID, input.UserID).Scan(&keyName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("api key not found")
	}
	if err != nil {
		return nil, fmt.Errorf("query api key: %w", err)
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO user_image_generations (
			user_id, api_key_id, api_key_name, model, size, prompt, status, images, reference_images
		) VALUES ($1, $2, $3, $4, $5, $6, 'processing', '[]'::jsonb, $7::jsonb)
		RETURNING id, user_id, api_key_id, api_key_name, model, size, prompt, status, images,
		          reference_images,
		          error_message, created_at, updated_at, completed_at
	`, input.UserID, input.APIKeyID, keyName, model, size, prompt, string(referenceImages))

	record, err := scanImageGenerationRecord(row)
	if err != nil {
		return nil, fmt.Errorf("create image generation: %w", err)
	}
	return &record, nil
}

func (s *ImageGenerationHistoryService) CreateAndStart(ctx context.Context, input CreateImageGenerationInput) (*ImageGenerationRecord, error) {
	record, err := s.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	go s.runImageGeneration(record.ID)
	return record, nil
}

func (s *ImageGenerationHistoryService) Update(ctx context.Context, input UpdateImageGenerationInput) (*ImageGenerationRecord, error) {
	status := strings.TrimSpace(input.Status)
	if status != string(ImageGenerationStatusProcessing) &&
		status != string(ImageGenerationStatusCompleted) &&
		status != string(ImageGenerationStatusFailed) &&
		status != string(ImageGenerationStatusCancelled) {
		return nil, errors.New("invalid status")
	}
	images := input.Images
	if len(images) == 0 {
		images = json.RawMessage("[]")
	}
	if !json.Valid(images) {
		return nil, errors.New("images must be valid json")
	}
	errorMessage := strings.TrimSpace(input.ErrorMessage)

	row := s.db.QueryRowContext(ctx, `
		UPDATE user_image_generations
		SET status = $1::text,
		    images = $2::jsonb,
		    error_message = $3,
		    updated_at = NOW(),
		    completed_at = CASE WHEN $1::text IN ('completed', 'failed', 'cancelled') THEN COALESCE(completed_at, NOW()) ELSE completed_at END
		WHERE id = $4 AND user_id = $5
		RETURNING id, user_id, api_key_id, api_key_name, model, size, prompt, status, images,
		          reference_images,
		          error_message, created_at, updated_at, completed_at
	`, status, string(images), errorMessage, input.ID, input.UserID)

	record, err := scanImageGenerationRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("image generation record not found")
	}
	if err != nil {
		return nil, fmt.Errorf("update image generation: %w", err)
	}
	return &record, nil
}

func (s *ImageGenerationHistoryService) Cancel(ctx context.Context, userID, id int64) (*ImageGenerationRecord, error) {
	if userID <= 0 {
		return nil, errors.New("user_id is required")
	}
	if id <= 0 {
		return nil, errors.New("image generation ID is required")
	}

	row := s.db.QueryRowContext(ctx, `
		UPDATE user_image_generations
		SET status = 'cancelled',
		    images = '[]'::jsonb,
		    error_message = '',
		    updated_at = NOW(),
		    completed_at = COALESCE(completed_at, NOW())
		WHERE id = $1 AND user_id = $2 AND status = 'processing'
		RETURNING id, user_id, api_key_id, api_key_name, model, size, prompt, status, images,
		          reference_images,
		          error_message, created_at, updated_at, completed_at
	`, id, userID)

	record, err := scanImageGenerationRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		record, loadErr := s.loadUserImageGenerationRecord(ctx, userID, id)
		if loadErr != nil {
			return nil, loadErr
		}
		return record, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cancel image generation: %w", err)
	}
	s.cancelActiveTask(id)
	return &record, nil
}

func (s *ImageGenerationHistoryService) loadUserImageGenerationRecord(ctx context.Context, userID, id int64) (*ImageGenerationRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, api_key_id, api_key_name, model, size, prompt, status, images,
		       reference_images,
		       error_message, created_at, updated_at, completed_at
		FROM user_image_generations
		WHERE id = $1 AND user_id = $2
	`, id, userID)

	record, err := scanImageGenerationRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("image generation record not found")
	}
	if err != nil {
		return nil, fmt.Errorf("load image generation record: %w", err)
	}
	return &record, nil
}

func (s *ImageGenerationHistoryService) runImageGeneration(recordID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	s.registerActiveTask(recordID, cancel)
	defer s.unregisterActiveTask(recordID)

	task, err := s.loadRunnableRecord(ctx, recordID)
	if err != nil {
		if errors.Is(err, errImageGenerationNotProcessing) || errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		_ = s.failRecord(context.Background(), recordID, 0, err.Error())
		return
	}

	images, err := s.callImagesGateway(ctx, task)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return
		}
		_ = s.failRecord(context.Background(), recordID, task.UserID, err.Error())
		return
	}
	if len(images) == 0 {
		_ = s.failRecord(context.Background(), recordID, task.UserID, "No images returned from API")
		return
	}

	raw, err := json.Marshal(images)
	if err != nil {
		_ = s.failRecord(context.Background(), recordID, task.UserID, "Failed to encode image results")
		return
	}
	_ = s.completeRecord(context.Background(), recordID, task.UserID, raw)
}

type runnableImageGenerationRecord struct {
	ID              int64
	UserID          int64
	APIKey          string
	Model           string
	Size            string
	Prompt          string
	Status          string
	ReferenceImages json.RawMessage
}

type storedImageResult struct {
	Src           string `json:"src"`
	RevisedPrompt string `json:"revisedPrompt,omitempty"`
}

func (s *ImageGenerationHistoryService) loadRunnableRecord(ctx context.Context, recordID int64) (*runnableImageGenerationRecord, error) {
	var task runnableImageGenerationRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT g.id, g.user_id, k.key, g.model, g.size, g.prompt, g.status, g.reference_images
		FROM user_image_generations g
		JOIN api_keys k ON k.id = g.api_key_id AND k.user_id = g.user_id AND k.deleted_at IS NULL
		WHERE g.id = $1
	`, recordID).Scan(&task.ID, &task.UserID, &task.APIKey, &task.Model, &task.Size, &task.Prompt, &task.Status, &task.ReferenceImages)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("image generation task not found")
	}
	if err != nil {
		return nil, fmt.Errorf("load image generation task: %w", err)
	}
	if task.Status != string(ImageGenerationStatusProcessing) {
		return nil, fmt.Errorf("%w: %s", errImageGenerationNotProcessing, task.Status)
	}
	return &task, nil
}

func (s *ImageGenerationHistoryService) callImagesGateway(ctx context.Context, task *runnableImageGenerationRecord) ([]storedImageResult, error) {
	references, err := decodeImageGenerationReferenceImages(task.ReferenceImages)
	if err != nil {
		return nil, err
	}

	requestPayload := map[string]any{
		"model":           task.Model,
		"prompt":          task.Prompt,
		"size":            task.Size,
		"n":               1,
		"response_format": "b64_json",
	}
	requestURL := s.localGenerationsURL
	if len(references) > 0 {
		images := make([]map[string]string, 0, len(references))
		for _, reference := range references {
			images = append(images, map[string]string{"image_url": reference.Src})
		}
		requestPayload["images"] = images
		requestURL = s.localEditsURL
	}

	body, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+task.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read image response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New(extractImageGatewayError(respBody, resp.StatusCode))
	}

	var payload struct {
		Data []struct {
			URL           string `json:"url"`
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, fmt.Errorf("decode image response: %w", err)
	}

	images := make([]storedImageResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		src := strings.TrimSpace(item.URL)
		if src == "" && strings.TrimSpace(item.B64JSON) != "" {
			src = "data:image/png;base64," + strings.TrimSpace(item.B64JSON)
		}
		if src == "" {
			continue
		}
		images = append(images, storedImageResult{
			Src:           src,
			RevisedPrompt: strings.TrimSpace(item.RevisedPrompt),
		})
	}
	return images, nil
}

func (s *ImageGenerationHistoryService) failRecord(ctx context.Context, recordID, userID int64, message string) error {
	if userID > 0 {
		_, err := s.db.ExecContext(ctx, `
			UPDATE user_image_generations
			SET status = 'failed',
			    images = '[]'::jsonb,
			    error_message = $1,
			    updated_at = NOW(),
			    completed_at = COALESCE(completed_at, NOW())
			WHERE id = $2 AND user_id = $3 AND status = 'processing'
		`, strings.TrimSpace(message), recordID, userID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE user_image_generations
		SET status = 'failed',
		    images = '[]'::jsonb,
		    error_message = $1,
		    updated_at = NOW(),
		    completed_at = COALESCE(completed_at, NOW())
		WHERE id = $2 AND status = 'processing'
	`, strings.TrimSpace(message), recordID)
	return err
}

func (s *ImageGenerationHistoryService) completeRecord(ctx context.Context, recordID, userID int64, images json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE user_image_generations
		SET status = 'completed',
		    images = $1::jsonb,
		    error_message = '',
		    updated_at = NOW(),
		    completed_at = COALESCE(completed_at, NOW())
		WHERE id = $2 AND user_id = $3 AND status = 'processing'
	`, string(images), recordID, userID)
	return err
}

func (s *ImageGenerationHistoryService) registerActiveTask(recordID int64, cancel context.CancelFunc) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	s.activeTasks[recordID] = cancel
}

func (s *ImageGenerationHistoryService) unregisterActiveTask(recordID int64) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	delete(s.activeTasks, recordID)
}

func (s *ImageGenerationHistoryService) cancelActiveTask(recordID int64) {
	s.activeMu.Lock()
	cancel := s.activeTasks[recordID]
	s.activeMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func extractImageGatewayError(body []byte, statusCode int) string {
	var payload struct {
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if strings.TrimSpace(payload.Error.Message) != "" {
			return payload.Error.Message
		}
		if strings.TrimSpace(payload.Message) != "" {
			return payload.Message
		}
	}
	return fmt.Sprintf("Image request failed with HTTP %d", statusCode)
}

type imageGenerationScanner interface {
	Scan(dest ...any) error
}

func scanImageGenerationRecord(scanner imageGenerationScanner) (ImageGenerationRecord, error) {
	var record ImageGenerationRecord
	var apiKeyID sql.NullInt64
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.UserID,
		&apiKeyID,
		&record.APIKeyName,
		&record.Model,
		&record.Size,
		&record.Prompt,
		&record.Status,
		&record.Images,
		&record.ReferenceImages,
		&record.ErrorMessage,
		&record.CreatedAt,
		&record.UpdatedAt,
		&completedAt,
	); err != nil {
		return ImageGenerationRecord{}, err
	}
	if apiKeyID.Valid {
		record.APIKeyID = &apiKeyID.Int64
	}
	if completedAt.Valid {
		record.CompletedAt = &completedAt.Time
	}
	if len(record.Images) == 0 {
		record.Images = json.RawMessage("[]")
	}
	if len(record.ReferenceImages) == 0 {
		record.ReferenceImages = json.RawMessage("[]")
	}
	return record, nil
}

type imageGenerationReferenceImage struct {
	Src         string `json:"src"`
	Name        string `json:"name,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

func normalizeImageGenerationReferenceImages(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage("[]"), nil
	}
	images, err := decodeImageGenerationReferenceImages(raw)
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return json.RawMessage("[]"), nil
	}
	normalized, err := json.Marshal(images)
	if err != nil {
		return nil, fmt.Errorf("encode reference images: %w", err)
	}
	return normalized, nil
}

func decodeImageGenerationReferenceImages(raw json.RawMessage) ([]imageGenerationReferenceImage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if !json.Valid(raw) {
		return nil, errors.New("reference_images must be valid json")
	}
	var images []imageGenerationReferenceImage
	if err := json.Unmarshal(raw, &images); err != nil {
		return nil, errors.New("reference_images must be an array")
	}
	if len(images) > maxImageGenerationReferenceImages {
		return nil, fmt.Errorf("reference_images supports at most %d images", maxImageGenerationReferenceImages)
	}

	totalBytes := int64(0)
	for i := range images {
		images[i].Src = strings.TrimSpace(images[i].Src)
		images[i].Name = strings.TrimSpace(images[i].Name)
		images[i].ContentType = strings.TrimSpace(images[i].ContentType)
		if images[i].Src == "" {
			return nil, errors.New("reference_images[].src is required")
		}
		if !isSupportedImageReferenceURL(images[i].Src) {
			return nil, errors.New("reference_images[].src must be a data:image URL or http image URL")
		}
		size := estimateImageReferenceBytes(images[i].Src, images[i].Size)
		if size > maxImageGenerationReferenceImageBytes {
			return nil, fmt.Errorf("reference image %d exceeds %dMB", i+1, maxImageGenerationReferenceImageBytes>>20)
		}
		if size > 0 {
			images[i].Size = size
			totalBytes += size
		}
	}
	if totalBytes > maxImageGenerationReferenceImagesBytes {
		return nil, fmt.Errorf("reference images exceed %dMB in total", maxImageGenerationReferenceImagesBytes>>20)
	}
	return images, nil
}

func isSupportedImageReferenceURL(src string) bool {
	lower := strings.ToLower(strings.TrimSpace(src))
	return strings.HasPrefix(lower, "data:image/") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://")
}

func estimateImageReferenceBytes(src string, declared int64) int64 {
	if declared > 0 {
		return declared
	}
	lower := strings.ToLower(src)
	if !strings.HasPrefix(lower, "data:image/") {
		return 0
	}
	idx := strings.Index(src, ",")
	if idx < 0 || idx+1 >= len(src) {
		return int64(len(src))
	}
	encoded := src[idx+1:]
	if strings.Contains(lower[:idx], ";base64") {
		if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
			return int64(len(decoded))
		}
		return int64(len(encoded) * 3 / 4)
	}
	return int64(len(encoded))
}
