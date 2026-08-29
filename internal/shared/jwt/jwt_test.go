package jwt

import (
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/platform/config"
)

func TestGenerateAndValidateRefreshTokenUsesConfiguredIssuer(t *testing.T) {
	oldConfig := config.Config
	defer func() { config.Config = oldConfig }()

	config.Config.JWT = config.JWTConfig{
		Secret:        "test-secret",
		Expiry:        time.Hour,
		Issuer:        "system",
		RefreshExpiry: 2 * time.Hour,
	}

	token, _, err := GenerateRefreshToken("user-123")
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	claims, err := ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken failed: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Fatalf("expected user-123, got %q", claims.UserID)
	}
	if claims.Issuer != "system" {
		t.Fatalf("expected issuer system, got %q", claims.Issuer)
	}
}

func TestRefreshTokenCannotValidateAsAccessToken(t *testing.T) {
	oldConfig := config.Config
	defer func() { config.Config = oldConfig }()
	config.Config.JWT = config.JWTConfig{Secret: "test-secret", Expiry: time.Hour, Issuer: "system", RefreshExpiry: time.Hour}
	token, _, err := GenerateRefreshToken("user-123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateToken(token); err == nil {
		t.Fatal("expected refresh token to be rejected as access token")
	}
}

func TestAccessTokenCannotValidateAsRefreshToken(t *testing.T) {
	oldConfig := config.Config
	defer func() { config.Config = oldConfig }()
	config.Config.JWT = config.JWTConfig{Secret: "test-secret", Expiry: time.Hour, Issuer: "system", RefreshExpiry: time.Hour}
	token, _, err := GenerateToken("user-123", "u@example.com", "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateRefreshToken(token); err == nil {
		t.Fatal("expected access token to be rejected as refresh token")
	}
}

func TestAccessTokenCarriesSessionID(t *testing.T) {
	oldConfig := config.Config
	defer func() { config.Config = oldConfig }()
	config.Config.JWT = config.JWTConfig{Secret: "test-secret", Expiry: time.Hour, Issuer: "system"}
	token, _, err := GenerateTokenForSession("user-123", "u@example.com", "user", "session-123")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateToken(token)
	if err != nil || claims.SessionID != "session-123" {
		t.Fatalf("session claim missing: %q, %v", claims.SessionID, err)
	}
}
