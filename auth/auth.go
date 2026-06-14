package auth

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"os"
	"strings"
	"time"
)

// ErrMissingTokenKey is returned when ENCRYPTION_TOKEN_KEY is unset/empty.
// Signing with an empty HMAC key, and verifying against one, are both trivially
// forgeable — so we fail closed (refuse to sign, reject on verify) rather than
// authenticate with a zero-byte key. Services should also guard this at
// startup via config.RequireEnv("ENCRYPTION_TOKEN_KEY").
var ErrMissingTokenKey = errors.New("ENCRYPTION_TOKEN_KEY is not set")

// secretKey reads ENCRYPTION_TOKEN_KEY at call time rather than at package
// init. Services load .env via godotenv.Load() inside main(), which runs
// AFTER package initialization — a package-level `var = os.Getenv(...)` would
// freeze an empty key and silently sign/verify every JWT with zero bytes.
func secretKey() []byte {
	return []byte(os.Getenv("ENCRYPTION_TOKEN_KEY"))
}

type UserClaims struct {
	jwt.RegisteredClaims
	Id    uint
	Email string
	Role  string
	Exp   int64
}

// createToken generate an access token
func createToken(user UserClaims) (string, error) {
	key := secretKey()
	if len(key) == 0 {
		return "", ErrMissingTokenKey
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"id":    user.Id,
			"email": user.Email,
			"role":  user.Role,
			"exp":   user.Exp,
		})

	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// VerifyToken verify an access token
func VerifyToken(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		key := secretKey()
		if len(key) == 0 {
			return nil, ErrMissingTokenKey
		}
		return key, nil
	})

	if err != nil {
		return err
	}

	if !token.Valid {
		return fmt.Errorf("invalid token")
	}

	return nil
}

type Tokens struct {
	AccessToken  string `form:"accessToken" json:"accessToken" xml:"accessToken"`
	RefreshToken string `form:"refreshToken" json:"refreshToken" xml:"refreshToken"`
}

// GenerateTokens generate refresh and access tokens
func GenerateTokens(id uint, email string, role string) (Tokens, error) {
	accessTokenDuration, _ := time.ParseDuration(os.Getenv("ACCESS_TOKEN_DURATION"))
	token, createError := createToken(UserClaims{
		Id:    id,
		Email: email,
		Role:  role,
		Exp:   time.Now().Add(accessTokenDuration).Unix(),
	})
	if createError != nil {
		return Tokens{}, createError
	}

	refreshTokenDuration, _ := time.ParseDuration(os.Getenv("REFRESH_TOKEN_DURATION"))
	refreshToken, createRefreshError := createToken(UserClaims{
		Id:    id,
		Email: email,
		Role:  role,
		Exp:   time.Now().Add(refreshTokenDuration).Unix(),
	})
	if createRefreshError != nil {
		return Tokens{}, createRefreshError
	}

	return Tokens{AccessToken: token, RefreshToken: refreshToken}, nil
}

func ParseToken(jwtToken string) (UserClaims, error) {
	var userClaim UserClaims
	token, err := jwt.ParseWithClaims(jwtToken, &userClaim, func(token *jwt.Token) (interface{}, error) {
		key := secretKey()
		if len(key) == 0 {
			return nil, ErrMissingTokenKey
		}
		return key, nil
	})
	if err != nil {
		return userClaim, err
	}

	// Checking token validity
	if !token.Valid {
		return userClaim, fmt.Errorf("invalid token")
	}

	return userClaim, nil
}

func ExtractJwtToken(header string) (string, error) {
	if header == "" {
		return "", fmt.Errorf("bad header value given")
	}

	jwtToken := strings.Split(header, " ")
	if len(jwtToken) != 2 {
		return "", fmt.Errorf("incorrectly formatted authorization header")
	}

	return jwtToken[1], nil
}
