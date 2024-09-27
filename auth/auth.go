package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

var secretKey = []byte("secret-key")

type UserClaims struct {
	jwt.RegisteredClaims
	Id    uint
	Email string
	Role  string
	Exp   int64
}

// createToken generate an access token
func createToken(user UserClaims) (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"id":    user.Id,
			"email": user.Email,
			"role":  user.Role,
			"exp":   time.Now().Add(time.Hour * 24 * 7).Unix(),
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

type Tokens struct {
	AccessToken  string `form:"accessToken" json:"accessToken" xml:"accessToken"`
	RefreshToken string `form:"refreshToken" json:"refreshToken" xml:"refreshToken"`
}

// GenerateTokens generate refresh and access tokens
func GenerateTokens(id uint, email string, role string) (Tokens, error) {
	token, createError := createToken(UserClaims{Id: id, Email: email, Role: role, Exp: time.Now().Add(time.Hour * 24).Unix()})
	if createError != nil {
		return Tokens{}, createError
	}
	refreshToken, createRefreshError := createToken(UserClaims{Id: id, Email: email, Exp: time.Now().Add(time.Hour * 24 * 7).Unix()})
	if createRefreshError != nil {
		return Tokens{}, createRefreshError
	}

	return Tokens{AccessToken: token, RefreshToken: refreshToken}, nil
}

func ParseToken(jwtToken string) (UserClaims, error) {
	var userClaim UserClaims
	token, err := jwt.ParseWithClaims(jwtToken, &userClaim, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
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
