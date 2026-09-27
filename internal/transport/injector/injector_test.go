package injector

import (
	"testing"

	"github.com/ak-repo/go-chat-system/internal/platform/config"
	"github.com/ak-repo/go-chat-system/internal/service"
)

func TestAccountDeliveryUsesDevelopmentAdapterOnlyInDevelopment(t *testing.T) {
	dev := accountDelivery("development", config.EmailConfig{AppURL: "http://localhost:5173"})
	if _, ok := dev.(*service.DevelopmentDelivery); !ok {
		t.Fatalf("development should use development delivery, got %T", dev)
	}
	prod := accountDelivery("production", config.EmailConfig{})
	if _, ok := prod.(*service.SMTPDelivery); !ok {
		t.Fatalf("non-development should require SMTP, got %T", prod)
	}
}

func TestConfiguredSMTPOverridesDevelopmentAdapter(t *testing.T) {
	delivery := accountDelivery("development", config.EmailConfig{SMTPHost: "smtp.example", SMTPPort: 587, From: "chat@example.com", AppURL: "https://chat.example"})
	smtpDelivery, ok := delivery.(*service.SMTPDelivery)
	if !ok || smtpDelivery.Host != "smtp.example" {
		t.Fatalf("configured SMTP was not selected: %#v", delivery)
	}
}
