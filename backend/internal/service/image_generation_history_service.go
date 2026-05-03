package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type ImageGenerationStatus string

const (
	ImageGenerationStatusProcessing ImageGenerationStatus = "processing"
	ImageGenerationStatusCompleted  ImageGenerationStatus = "completed"
	ImageGenerationStatusFailed     ImageGenerationStatus = "failed"
)

var ErrImageGenerationAlreadyProcessing = errors.New("current image generation is still processing")

type ImageGenerationRecord struct {
	ID           int64           `json:"id"`
	UserID       int64           `json:"user_id"`
	APIKeyID     *int64          `json:"api_key_id,omitempty"`
	APIKeyName   string          `json:"api_key_name"`
	Model        string          `json:"model"`
	Size         string          `json:"size"`
	Prompt       string          `json:"prompt"`
	Status       string          `json:"status"`
	Images       json.RawMessage `json:"images"`
	ErrorMessage string          `json:"error_message"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
}

type CreateImageGenerationInput struct {
	UserID   int64
	APIKeyID int64
	Model    string
	Size     string
	Prompt   string
}

type UpdateImageGenerationInput struct {
	UserID       int64
	ID           int64
	Status       string
	Images       json.RawMessage
	ErrorMessage string
}

type ImageGenerationHistoryService struct {
	db         *sql.DB
	httpClient *http.Client
	localURL   string
}

func NewImageGenerationHistoryService(db *sql.DB, cfg *config.Config) *ImageGenerationHistoryService {
	port := 8080
	if cfg != nil && cfg.Server.Port > 0 {
		port = cfg.Server.Port
	}
	return &ImageGenerationHistoryService{
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Minute},
		localURL:   fmt.Sprintf("http://127.0.0.1:%d/v1/images/generations", port),
	}
}

func (s *ImageGenerationHistoryService) List(ctx context.Context, userID int64, limit int) ([]ImageGenerationRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, api_key_id, api_key_name, model, size, prompt, status, images,
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
	err := s.db.QueryRowContext(ctx, `
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
			user_id, api_key_id, api_key_name, model, size, prompt, status, images
		) VALUES ($1, $2, $3, $4, $5, $6, 'processing', '[]'::jsonb)
		RETURNING id, user_id, api_key_id, api_key_name, model, size, prompt, status, images,
		          error_message, created_at, updated_at, completed_at
	`, input.UserID, input.APIKeyID, keyName, model, size, prompt)

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
		status != string(ImageGenerationStatusFailed) {
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
		    completed_at = CASE WHEN $1::text IN ('completed', 'failed') THEN COALESCE(completed_at, NOW()) ELSE completed_at END
		WHERE id = $4 AND user_id = $5
		RETURNING id, user_id, api_key_id, api_key_name, model, size, prompt, status, images,
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

func (s *ImageGenerationHistoryService) runImageGeneration(recordID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	task, err := s.loadRunnableRecord(ctx, recordID)
	if err != nil {
		_ = s.failRecord(context.Background(), recordID, 0, err.Error())
		return
	}

	images, err := s.callImagesGateway(ctx, task)
	if err != nil {
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
	_, _ = s.Update(context.Background(), UpdateImageGenerationInput{
		UserID: task.UserID,
		ID:     recordID,
		Status: string(ImageGenerationStatusCompleted),
		Images: raw,
	})
}

type runnableImageGenerationRecord struct {
	ID     int64
	UserID int64
	APIKey string
	Model  string
	Size   string
	Prompt string
	Status string
}

type storedImageResult struct {
	Src           string `json:"src"`
	RevisedPrompt string `json:"revisedPrompt,omitempty"`
}

func (s *ImageGenerationHistoryService) loadRunnableRecord(ctx context.Context, recordID int64) (*runnableImageGenerationRecord, error) {
	var task runnableImageGenerationRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT g.id, g.user_id, k.key, g.model, g.size, g.prompt, g.status
		FROM user_image_generations g
		JOIN api_keys k ON k.id = g.api_key_id AND k.user_id = g.user_id AND k.deleted_at IS NULL
		WHERE g.id = $1
	`, recordID).Scan(&task.ID, &task.UserID, &task.APIKey, &task.Model, &task.Size, &task.Prompt, &task.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("image generation task not found")
	}
	if err != nil {
		return nil, fmt.Errorf("load image generation task: %w", err)
	}
	if task.Status != string(ImageGenerationStatusProcessing) {
		return nil, fmt.Errorf("image generation task is already %s", task.Status)
	}
	return &task, nil
}

func (s *ImageGenerationHistoryService) callImagesGateway(ctx context.Context, task *runnableImageGenerationRecord) ([]storedImageResult, error) {
	body, err := json.Marshal(map[string]any{
		"model":           task.Model,
		"prompt":          task.Prompt,
		"size":            task.Size,
		"n":               1,
		"response_format": "b64_json",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.localURL, bytes.NewReader(body))
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
		_, err := s.Update(ctx, UpdateImageGenerationInput{
			UserID:       userID,
			ID:           recordID,
			Status:       string(ImageGenerationStatusFailed),
			Images:       json.RawMessage("[]"),
			ErrorMessage: strings.TrimSpace(message),
		})
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE user_image_generations
		SET status = 'failed',
		    images = '[]'::jsonb,
		    error_message = $1,
		    updated_at = NOW(),
		    completed_at = COALESCE(completed_at, NOW())
		WHERE id = $2
	`, strings.TrimSpace(message), recordID)
	return err
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
	return record, nil
}
