package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"net/http"
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
			raw, h, _ := token()
			_, _ = s.tokens.CreateAccountToken(r.Context(), u.ID, "password_reset", h, time.Now().Add(time.Hour))
			_ = s.delivery.Deliver("password_reset", u.Email, raw)
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
		if u, e := s.users.GetByEmail(r.Context(), q.Email); e == nil && u != nil {
			raw, h, _ := token()
			_, _ = s.tokens.CreateAccountToken(r.Context(), u.ID, "verification", h, time.Now().Add(24*time.Hour))
			_ = s.delivery.Deliver("verification", u.Email, raw)
		}
	}
	return 200, utils.SuccessResponse(map[string]string{"message": "If the account exists, verification instructions will be sent."}), nil
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
