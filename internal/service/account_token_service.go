package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

type Delivery interface {
	Deliver(string, string, string) error
}
type DevelopmentDelivery struct {
	mu                                  sync.Mutex
	LastPurpose, LastAddress, LastToken string
}

type SMTPDelivery struct {
	Host                             string
	Port                             int
	Username, Password, From, AppURL string
}

func (d *SMTPDelivery) Deliver(purpose, address, raw string) error {
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
	var auth smtp.Auth
	if d.Username != "" {
		auth = smtp.PlainAuth("", d.Username, d.Password, d.Host)
	}
	return smtp.SendMail(fmt.Sprintf("%s:%d", d.Host, d.Port), auth, d.From, []string{address}, message)
}

func (d *DevelopmentDelivery) Deliver(p, a, t string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.LastPurpose = p
	d.LastAddress = a
	d.LastToken = t
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
			} else if deliveryErr := s.delivery.Deliver("password_reset", u.Email, raw); deliveryErr != nil {
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
	return s.delivery.Deliver("verification", u.Email, raw)
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
