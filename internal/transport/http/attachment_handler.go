package httptransport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/internal/service"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/go-chi/chi"
)

type AttachmentHandler struct{ Service *service.AttachmentService }

func attachmentResult(data any, err error) (int, *utils.APIResponse, error) {
	if err == nil {
		return 200, utils.SuccessResponse(data), nil
	}
	status := 500
	switch {
	case errors.Is(err, errs.ErrValidation):
		status = 400
	case errors.Is(err, errs.ErrUnauthorized):
		status = 401
	case errors.Is(err, errs.ErrForbidden), errors.Is(err, errs.ErrNotMember), errors.Is(err, errs.ErrBlockedRelationship):
		status = 403
	case errors.Is(err, errs.ErrNotFound):
		status = 404
	case errors.Is(err, errs.ErrConflict):
		status = 409
	case errors.Is(err, errs.ErrUnavailable):
		status = 503
	}
	return status, nil, err
}
func attachmentActor(r *http.Request) string {
	v, _ := r.Context().Value(middleware.UserIDKey).(string)
	return v
}
func (h *AttachmentHandler) Initiate(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var q struct {
		Filename string `json:"filename"`
		MIME     string `json:"mime_type"`
		Size     int64  `json:"size_bytes"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil {
		return attachmentResult(nil, errs.ErrValidation)
	}
	a, u, err := h.Service.Initiate(r.Context(), attachmentActor(r), chi.URLParam(r, "conversationID"), q.Filename, q.MIME, q.Size)
	if err != nil {
		return attachmentResult(nil, err)
	}
	return 201, utils.SuccessResponse(map[string]any{"attachment": a, "upload": u, "expires_at": a.UploadExpiresAt}), nil
}
func (h *AttachmentHandler) Status(_ http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	a, err := h.Service.Status(r.Context(), attachmentActor(r), chi.URLParam(r, "attachmentID"))
	return attachmentResult(a, err)
}
func (h *AttachmentHandler) Complete(_ http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	a, err := h.Service.Complete(r.Context(), attachmentActor(r), chi.URLParam(r, "attachmentID"))
	status, data, err := attachmentResult(a, err)
	if err == nil && a.State != "ready" {
		status = 202
	}
	return status, data, err
}
func (h *AttachmentHandler) Cancel(_ http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	return attachmentResult(nil, h.Service.Cancel(r.Context(), attachmentActor(r), chi.URLParam(r, "attachmentID")))
}
func (h *AttachmentHandler) URL(_ http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	variant := r.URL.Query().Get("variant")
	if variant == "" {
		variant = "full"
	}
	download := r.URL.Query().Get("download")
	if download != "" && download != "true" && download != "false" {
		return attachmentResult(nil, errs.ErrValidation)
	}
	url, err := h.Service.URL(r.Context(), attachmentActor(r), chi.URLParam(r, "messageID"), chi.URLParam(r, "attachmentID"), variant, download == "true")
	return attachmentResult(map[string]any{"url": url, "expires_at": time.Now().UTC().Add(5 * time.Minute)}, err)
}

func (h *AttachmentHandler) Capabilities(_ http.ResponseWriter, _ *http.Request) (int, *utils.APIResponse, error) {
	return attachmentResult(map[string]any{
		"images_enabled": h.Service.Enabled(),
		"mime_types":     []string{"image/jpeg", "image/png", "image/webp"},
		"max_bytes":      service.MaxImageBytes, "max_pixels": service.MaxImagePixels,
		"max_attachments": 1,
	}, nil)
}
