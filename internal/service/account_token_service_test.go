package service

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
)

type fakeDelivery struct {
	purpose, address, raw string
	err                   error
}

type accountTokenRecorder struct {
	*fakeUserRepo
	purpose   string
	verified  bool
	verifyErr error
}

func (f *accountTokenRecorder) CreateAccountToken(ctx context.Context, uid, purpose string, hash []byte, expiry time.Time) (string, error) {
	f.purpose = purpose
	return f.fakeUserRepo.CreateAccountToken(ctx, uid, purpose, hash, expiry)
}
func (f *accountTokenRecorder) VerifyAccount(context.Context, []byte) (string, error) {
	f.verified = true
	return "user-1", f.verifyErr
}

func (f *fakeDelivery) Deliver(_ context.Context, purpose, address, raw string) error {
	f.purpose, f.address, f.raw = purpose, address, raw
	return f.err
}

func TestDevelopmentDeliveryLogsUsableLink(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	delivery := &DevelopmentDelivery{AppURL: "http://localhost:5173/"}
	if err := delivery.Deliver(context.Background(), "verification", "alice@example.com", "secret-token"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "DEVELOPMENT ONLY") || !strings.Contains(output.String(), "http://localhost:5173/verify?token=secret-token") {
		t.Fatalf("development verification link not logged: %q", output.String())
	}
	output.Reset()
	if err := delivery.Deliver(context.Background(), "password_reset", "alice@example.com", "reset-token"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "http://localhost:5173/recover?token=reset-token") {
		t.Fatalf("development reset link not logged: %q", output.String())
	}
}

func TestSMTPDeliveryTimesOutWaitingForServerGreeting(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, e := listener.Accept()
		if e == nil {
			defer conn.Close()
			_, _ = conn.Read(make([]byte, 1))
		}
	}()
	address, _ := listener.Addr().(*net.TCPAddr)
	delivery := &SMTPDelivery{Host: "127.0.0.1", Port: address.Port, From: "chat@example.com", AppURL: "http://localhost", Timeout: 100 * time.Millisecond}
	started := time.Now()
	err = delivery.Deliver(context.Background(), "verification", "alice@example.com", "secret")
	if err == nil {
		t.Fatal("expected SMTP greeting timeout")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("SMTP timeout was not bounded: %s", elapsed)
	}
}

func TestRequestResetIsDeliberatelyGenericAndDeliversToken(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: "user-1", Email: "alice@example.com"}}
	delivery := &fakeDelivery{}
	status, response, err := NewAccountTokenService(repo, delivery).RequestReset(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":" ALICE@EXAMPLE.COM "}`)))
	if status != http.StatusOK || response == nil || err != nil {
		t.Fatalf("expected generic reset response, got %d, %v", status, err)
	}
	if delivery.purpose != "password_reset" || delivery.address != "alice@example.com" || delivery.raw == "" {
		t.Fatalf("expected delivered reset token, got %#v", delivery)
	}
}

func TestResetPasswordConsumesTokenAndChangesPassword(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: "user-1"}}
	status, _, err := NewAccountTokenService(repo, &fakeDelivery{}).ResetPassword(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"token":"raw-token","password":"strong-password"}`)))
	if status != http.StatusOK || err != nil || repo.passwordID != "user-1" || repo.passwordHash == "" {
		t.Fatalf("expected password reset, got %d, %v", status, err)
	}
}

func TestResetPasswordValidatesAndMapsTokenErrors(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: "user-1"}}
	service := NewAccountTokenService(repo, &fakeDelivery{})
	status, _, err := service.ResetPassword(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"token":"","password":"strong-password"}`)))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("expected validation error, got %d, %v", status, err)
	}
	service.tokens = failingAccountTokenRepo{}
	status, _, err = service.ResetPassword(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"token":"raw-token","password":"strong-password"}`)))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected unauthorized token error, got %d, %v", status, err)
	}
}

func TestRequestVerificationAndVerify(t *testing.T) {
	repo := &accountTokenRecorder{fakeUserRepo: &fakeUserRepo{user: &model.User{ID: "user-1", Email: "alice@example.com"}}}
	delivery := &fakeDelivery{}
	service := NewAccountTokenService(repo, delivery)
	status, _, err := service.RequestVerification(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":" ALICE@EXAMPLE.COM "}`)))
	if status != http.StatusOK || err != nil || repo.purpose != "verification" || delivery.purpose != "verification" {
		t.Fatalf("expected verification delivery, got %d %v", status, err)
	}
	status, _, err = service.Verify(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"token":"raw-token"}`)))
	if status != http.StatusOK || err != nil || !repo.verified {
		t.Fatalf("expected verification, got %d %v", status, err)
	}
	repo.verifyErr = errors.New("expired")
	status, _, err = service.Verify(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"token":"raw-token"}`)))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected invalid verification token, got %d %v", status, err)
	}
	status, _, err = service.Verify(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`)))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("expected verification validation, got %d %v", status, err)
	}
}

type failingAccountTokenRepo struct{}

func (failingAccountTokenRepo) CreateAccountToken(context.Context, string, string, []byte, time.Time) (string, error) {
	return "", nil
}
func (failingAccountTokenRepo) ConsumeAccountToken(context.Context, string, []byte) (string, error) {
	return "", errors.New("expired")
}
func (failingAccountTokenRepo) VerifyAccount(context.Context, []byte) (string, error) { return "", nil }
