package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ImageGenerationHandler struct {
	service *service.ImageGenerationHistoryService
}

func NewImageGenerationHandler(service *service.ImageGenerationHistoryService) *ImageGenerationHandler {
	return &ImageGenerationHandler{service: service}
}

type createImageGenerationRequest struct {
	APIKeyID        int64           `json:"api_key_id" binding:"required"`
	Model           string          `json:"model" binding:"required"`
	Size            string          `json:"size"`
	Prompt          string          `json:"prompt" binding:"required"`
	ReferenceImages json.RawMessage `json:"reference_images"`
}

type updateImageGenerationRequest struct {
	Status       string          `json:"status" binding:"required"`
	Images       json.RawMessage `json:"images"`
	ErrorMessage string          `json:"error_message"`
}

func (h *ImageGenerationHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	records, err := h.service.List(c.Request.Context(), subject.UserID, limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, records)
}

func (h *ImageGenerationHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req createImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	record, err := h.service.CreateAndStart(c.Request.Context(), service.CreateImageGenerationInput{
		UserID:          subject.UserID,
		APIKeyID:        req.APIKeyID,
		Model:           req.Model,
		Size:            req.Size,
		Prompt:          req.Prompt,
		ReferenceImages: req.ReferenceImages,
	})
	if err != nil {
		if errors.Is(err, service.ErrImageGenerationAlreadyProcessing) {
			response.Error(c, http.StatusConflict, "Current image generation is still processing")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, record)
}

func (h *ImageGenerationHandler) Update(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid image generation ID")
		return
	}

	var req updateImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	record, err := h.service.Update(c.Request.Context(), service.UpdateImageGenerationInput{
		UserID:       subject.UserID,
		ID:           id,
		Status:       req.Status,
		Images:       req.Images,
		ErrorMessage: req.ErrorMessage,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, record)
}

func (h *ImageGenerationHandler) Cancel(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid image generation ID")
		return
	}

	record, err := h.service.Cancel(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, record)
}
