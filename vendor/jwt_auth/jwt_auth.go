package jwt_auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserId uint `json:"user_id"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}

var secretKey = []byte("secret-key")

func GenerateJwtToken(userId uint, userEmail string) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		&Claims{
			UserId: userId,
			Email: userEmail,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(60 * time.Minute)),
				IssuedAt: jwt.NewNumericDate(time.Now()),
			},
		},
	)

	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", errors.New("Couldn't generate auth token")
	}

	return tokenString, nil
}

func ValidateToken(tokenString string) *Claims {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return secretKey, nil
	})

	if err != nil {
		return nil
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims
	}

	return nil
}
