package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hashed_password, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed_password), nil
}

func CheckPasswordHash(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err

}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	currentTime := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(currentTime),
		ExpiresAt: jwt.NewNumericDate(currentTime.Add(expiresIn)),
		Subject:   userID.String(),
	}

	new_token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed_string, err := new_token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return signed_string, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// This is the key function - it returns the secret key
		return []byte(tokenSecret), nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	// Check if token is valid
	if !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token")
	}

	// Extract and convert the subject to UUID
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	token_string := headers.Get("Authorization")
	if token_string == "" {
		return "", fmt.Errorf("no authorization header provided")
	}

	parts := strings.Split(token_string, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("malformed authorization header")
	}

	return parts[1], nil
}

func MakeRefreshToken() (string, error) {
	random_data := make([]byte, 32)
	_, err := rand.Read(random_data)
	if err != nil {
		return "", err
	}
	hex_string := hex.EncodeToString(random_data)
	return hex_string, nil
}
