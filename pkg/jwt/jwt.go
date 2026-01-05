package jwt

import (
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secretKey []byte
	tokenTTL  time.Duration
}

func NewJWTManager(secret string, exp time.Duration) *JWTManager {
	return &JWTManager{
		secretKey: []byte(secret),
		tokenTTL:  exp,
	}
}

// GenerateAccessToken creates a new JWT access token
func (m *JWTManager) GenerateToken(userID, email, role string) (string, time.Time, error) {
	expirationTime := time.Now().Add(m.tokenTTL)
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secretKey, nil
	})

	if err != nil {
		log.Println("called: ", err.Error())

		return nil, err // keep raw error during debugging
	}

	if !token.Valid {

		return nil, fmt.Errorf("token invalid")
	}

	return claims, nil
}
