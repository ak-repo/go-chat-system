package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"net/http"
)

type Delivery interface {
	Deliver(context.Context, string, string, string) error
}
type DevelopmentDelivery struct {
	mu                                  sync.Mutex
	LastPurpose, LastAddress, LastToken string
	AppURL                              string
}

type SMTPDelivery struct {
	Host                             string
	Port                             int
	Username, Password, From, AppURL string
	Timeout                          time.Duration
}

const defaultSMTPTimeout = 8 * time.Second

func (d *SMTPDelivery) Deliver(parent context.Context, purpose, address, raw string) error {
	if d.Host == "" || d.Port <= 0 || d.From == "" || d.AppURL == "" {
		return fmt.Errorf("email delivery is not configured")
	}
	link := strings.TrimRight(d.AppURL, "/") + "/verify?token=" + raw
	if purpose == "password_reset" {
		link = strings.TrimRight(d.AppURL, "/") + "/recover?token=" + raw
	}
	subject := "Verify your chat account"
	if purpose == "password_reset" {
		subject = "Reset your chat password"
	}
	body := fmt.Sprintf("To continue, open this link:\r\n\r\n%s\r\n\r\nIf you did not request this, ignore this message.\r\n", link)
	message := []byte("To: " + address + "\r\nFrom: " + d.From + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = defaultSMTPTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", net.JoinHostPort(d.Host, fmt.Sprint(d.Port)))
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer conn.Close()
	stopCancellation := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancellation()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set SMTP deadline: %w", err)
		}
	}
	client, err := smtp.NewClient(conn, d.Host)
	if err != nil {
		return fmt.Errorf("start SMTP session: %w", err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: d.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	} else if d.Username != "" {
		return fmt.Errorf("SMTP server does not support STARTTLS")
	}
	var auth smtp.Auth
	if d.Username != "" {
		auth = smtp.PlainAuth("", d.Username, d.Password, d.Host)
	}
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authenticate to SMTP server: %w", err)
		}
	}
	if err := client.Mail(d.From); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP message: %w", err)
	}
	if _, err := io.WriteString(writer, string(message)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("send SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}
	return nil
}

func (d *DevelopmentDelivery) Deliver(ctx context.Context, p, a, t string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.LastPurpose = p
	d.LastAddress = a
	d.LastToken = t
	path := "/verify?token="
	if p == "password_reset" {
		path = "/recover?token="
	}
	log.Printf("DEVELOPMENT ONLY %s link (send this to %s): %s%s%s", p, a, strings.TrimRight(d.AppURL, "/"), path, t)
	return nil
}

type AccountTokenService struct {
	users    repository.UserRepository
	tokens   repository.AccountTokenRepository
	accounts repository.UserAccountRepository
	delivery Delivery
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func NewAccountTokenService(u repository.UserRepository, d Delivery) *AccountTokenService {
	return &AccountTokenService{users: u, tokens: u.(repository.AccountTokenRepository), accounts: u.(repository.UserAccountRepository), delivery: d, attempts: map[string][]time.Time{}}
}
func (s *AccountTokenService) allowed(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if _, exists := s.attempts[key]; !exists && len(s.attempts) >= 10000 {
		return false
	}
	var a []time.Time
	for _, t := range s.attempts[key] {
		if now.Sub(t) < time.Hour {
			a = append(a, t)
		}
	}
	if len(s.attempts) > 10000 {
		for k, times := range s.attempts {
			if len(times) == 0 || now.Sub(times[len(times)-1]) >= time.Hour {
				delete(s.attempts, k)
			}
		}
	}
	if len(a) >= 5 {
		s.attempts[key] = a
		return false
	}
	s.attempts[key] = append(a, now)
	return true
}
func token() (string, []byte, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", nil, e
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(b), h[:], nil
}
func (s *AccountTokenService) RequestReset(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var q struct {
		Email string `json:"email"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q)
	q.Email = strings.ToLower(strings.TrimSpace(q.Email))
	if len(q.Email) <= utils.MaxEmailLength && q.Email != "" && s.allowed(q.Email) {
		if u, e := s.users.GetByEmail(r.Context(), q.Email); e == nil && u != nil {
			raw, h, tokenErr := token()
			if tokenErr != nil {
				log.Printf("password reset token generation failed: %v", tokenErr)
			} else if _, createErr := s.tokens.CreateAccountToken(r.Context(), u.ID, "password_reset", h, time.Now().Add(time.Hour)); createErr != nil {
				log.Printf("password reset token persistence failed: %v", createErr)
			} else if deliveryErr := s.delivery.Deliver(r.Context(), "password_reset", u.Email, raw); deliveryErr != nil {
				log.Printf("password reset delivery failed: %v", deliveryErr)
			}
		}
	}
	return 200, utils.SuccessResponse(map[string]string{"message": "If the account exists, recovery instructions will be sent."}), nil
}
func (s *AccountTokenService) ResetPassword(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var q struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q) != nil || len(q.Token) > 256 || q.Token == "" || !utils.ValidatePassword(q.Password) {
		return 400, nil, errs.ErrValidation
	}
	h := sha256.Sum256([]byte(q.Token))
	uid, e := s.tokens.ConsumeAccountToken(r.Context(), "password_reset", h[:])
	if e != nil {
		return 400, nil, errs.ErrUnauthorized
	}
	ph, e := utils.HashPassword(q.Password)
	if e != nil {
		return 500, nil, errs.ErrInternal
	}
	if pr, ok := s.users.(repository.PasswordResetRepository); ok {
		if e = pr.ChangePasswordAndRevokeSessions(r.Context(), uid, ph); e != nil {
			return 500, nil, errs.ErrInternal
		}
	} else {
		if e = s.accounts.ChangePassword(r.Context(), uid, ph); e != nil {
			return 500, nil, errs.ErrInternal
		}
		if sr, ok := s.users.(repository.SessionRepository); ok {
			if e = sr.RevokeUserSessions(r.Context(), uid); e != nil {
				return 500, nil, errs.ErrInternal
			}
		}
	}
	return 200, utils.SuccessResponse(map[string]string{"status": "reset"}), nil
}
func (s *AccountTokenService) RequestVerification(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var q struct {
		Email string `json:"email"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q)
	q.Email = strings.ToLower(strings.TrimSpace(q.Email))
	if len(q.Email) <= utils.MaxEmailLength && q.Email != "" && s.allowed("verify:"+q.Email) {
		if err := s.sendVerification(r.Context(), q.Email); err != nil {
			log.Printf("verification delivery failed: %v", err)
		}
	}
	return 200, utils.SuccessResponse(map[string]string{"message": "If the account exists, verification instructions will be sent."}), nil
}

func (s *AccountTokenService) SendVerification(ctx context.Context, email string) error {
	return s.sendVerification(ctx, email)
}
func (s *AccountTokenService) sendVerification(ctx context.Context, email string) error {
	u, err := s.users.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || u == nil {
		return err
	}
	if u.VerifiedAt != nil {
		return nil
	}
	raw, h, err := token()
	if err != nil {
		return err
	}
	if _, err = s.tokens.CreateAccountToken(ctx, u.ID, "verification", h, time.Now().Add(24*time.Hour)); err != nil {
		return err
	}
	return s.delivery.Deliver(ctx, "verification", u.Email, raw)
}
func (s *AccountTokenService) Verify(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var q struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.Token == "" {
		return 400, nil, errs.ErrValidation
	}
	h := sha256.Sum256([]byte(q.Token))
	if _, e := s.tokens.VerifyAccount(r.Context(), h[:]); e != nil {
		return 400, nil, errs.ErrUnauthorized
	}
	return 200, utils.SuccessResponse(map[string]string{"status": "verified"}), nil
}
