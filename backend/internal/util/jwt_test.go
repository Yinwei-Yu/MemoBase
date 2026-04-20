package util

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignToken(t *testing.T) {
	t.Parallel()

	tokenString, err := SignToken("secret", "u_123", 2*time.Hour)
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("parsed token invalid: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims type = %T; want jwt.MapClaims", token.Claims)
	}
	if claims["sub"] != "u_123" {
		t.Fatalf("sub = %v; want %q", claims["sub"], "u_123")
	}
}
