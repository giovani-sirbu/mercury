package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

var secretKey = []byte("secret-key")

// createToken generate an access token
func createToken(id string, email string, role string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"id":    id,
			"email": email,
			"role":  role,
			"exp":   time.Now().Add(time.Hour * 24).Unix(),
		})

	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// VerifyToken verify an access token
func VerifyToken(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})

	if err != nil {
		return err
	}

	if !token.Valid {
		return fmt.Errorf("invalid token")
	}

	return nil
}

// createRefreshToken generate an access token used to refresh the access token
func createRefreshToken(email string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"exp": time.Now().Add(time.Hour * 24 * 7).Unix(),
		})

	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

type Tokens struct {
	Token        string
	RefreshToken string
}

// GenerateTokens generate refresh and access tokens
func GenerateTokens(id string, email string, role string) (Tokens, error) {
	token, createError := createToken(id, email, role)
	if createError != nil {
		return Tokens{}, createError
	}
	refreshToken, createRefreshError := createRefreshToken(email)
	if createRefreshError != nil {
		return Tokens{}, createRefreshError
	}

	return Tokens{Token: token, RefreshToken: refreshToken}, nil
}
