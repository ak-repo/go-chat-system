package httptransport

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/ak-repo/go-chat-system/internal/service"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
)

func TestDisabledMediaCapabilities(t *testing.T) {
	h := &AttachmentHandler{Service: service.NewAttachmentService(nil, nil, nil)}
	status, response, err := h.Capabilities(httptest.NewRecorder(), httptest.NewRequest("GET", "/media/capabilities", nil))
	if status != 200 || err != nil {
		t.Fatalf("capabilities: %d %v", status, err)
	}
	data, _ := json.Marshal(response)
	var envelope struct {
		Data struct {
			Enabled  bool  `json:"images_enabled"`
			MaxBytes int64 `json:"max_bytes"`
			Count    int   `json:"max_attachments"`
		} `json:"data"`
	}
	if err = json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Enabled || envelope.Data.MaxBytes != service.MaxImageBytes || envelope.Data.Count != 1 {
		t.Fatalf("capabilities: %s", data)
	}
}
func TestAttachmentErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{errs.ErrValidation, 400}, {errs.ErrUnauthorized, 401}, {errs.ErrForbidden, 403}, {errs.ErrNotMember, 403}, {errs.ErrBlockedRelationship, 403}, {errs.ErrNotFound, 404}, {errs.ErrConflict, 409}, {errs.ErrUnavailable, 503}, {errors.New("storage"), 500}} {
		status, _, err := attachmentResult(nil, tc.err)
		if status != tc.status || !errors.Is(err, tc.err) {
			t.Errorf("%v: %d", tc.err, status)
		}
	}
}
