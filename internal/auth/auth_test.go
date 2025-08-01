package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"
	expiresIn := time.Hour

	// Create a JWT
	token, err := MakeJWT(userID, secret, expiresIn)
	if err != nil {
		t.Fatal("Failed to create JWT:", err)
	}

	// Validate the JWT
	validatedID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatal("Failed to validate JWT:", err)
	}

	// Check that the user ID matches
	if validatedID != userID {
		t.Errorf("Expected user ID %v, got %v", userID, validatedID)
	}
}

func TestExpiredJWT(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"
	shortExpiration := time.Millisecond * 10

	// Create a JWT with short expiration
	token, err := MakeJWT(userID, secret, shortExpiration)
	if err != nil {
		t.Fatal("Failed to create JWT:", err)
	}

	// Wait for the token to expire
	time.Sleep(time.Millisecond * 20)

	// Try to validate the expired token
	_, err = ValidateJWT(token, secret)
	if err == nil {
		t.Error("Expected validation to fail for expired token, but it succeeded")
	}
}

func TestWrongSecret(t *testing.T) {
	// Test that JWTs signed with wrong secret are rejected
	userID := uuid.New()
	secret := "test-secret"
	wrongSecret := "wrong-secret"
	expiresIn := time.Hour

	// Create a JWT
	token, err := MakeJWT(userID, secret, expiresIn)
	if err != nil {
		t.Fatal("Failed to create JWT:", err)
	}

	// Attempt to validate with wrong secret
	_, err = ValidateJWT(token, wrongSecret)
	if err == nil {
		t.Error("Expected validation to fail with wrong secret, but it succeeded")
	}
}
