package util

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignToken_EmptySecret(t *testing.T) {
	t.Parallel()

	token, err := SignToken("", "u_123", time.Hour)
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	if token == "" {
		t.Fatalf("token should not be empty")
	}
}

func TestSignToken_EmptyUserID(t *testing.T) {
	t.Parallel()

	token, err := SignToken("secret", "", time.Hour)
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != "" {
		t.Fatalf("sub = %v; want empty string", claims["sub"])
	}
}

func TestSignToken_ShortTTL(t *testing.T) {
	t.Parallel()

	token, err := SignToken("secret", "u_123", 1*time.Second)
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	exp := int64(claims["exp"].(float64))
	iat := int64(claims["iat"].(float64))
	if exp-iat != 1 {
		t.Fatalf("exp-iat = %d; want 1", exp-iat)
	}
}

func TestSignToken_InvalidSignature(t *testing.T) {
	t.Parallel()

	token, err := SignToken("correct-secret", "u_123", time.Hour)
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	_, err = jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte("wrong-secret"), nil
	})
	if err == nil {
		t.Fatalf("expected signature verification error")
	}
}

func TestSignToken_ExpiredToken(t *testing.T) {
	t.Parallel()

	token, err := SignToken("secret", "u_123", -1*time.Hour)
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	_, err = jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if err == nil {
		t.Fatalf("expected expired token error")
	}
}

func TestSignToken_MalformedToken(t *testing.T) {
	t.Parallel()

	_, err := jwt.Parse("not.a.valid.token", func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if err == nil {
		t.Fatalf("expected parse error for malformed token")
	}
}

func TestSignToken_DifferentSecretsProduceDifferentTokens(t *testing.T) {
	t.Parallel()

	token1, _ := SignToken("secret1", "u_123", time.Hour)
	token2, _ := SignToken("secret2", "u_123", time.Hour)
	if token1 == token2 {
		t.Fatalf("tokens with different secrets should differ")
	}
}

func TestSignToken_IssuedAtIsRecent(t *testing.T) {
	t.Parallel()

	before := time.Now().Unix()
	token, err := SignToken("secret", "u_123", time.Hour)
	if err != nil {
		t.Fatalf("SignToken() error = %v", err)
	}
	after := time.Now().Unix()

	parsed, _ := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	claims := parsed.Claims.(jwt.MapClaims)
	iat := int64(claims["iat"].(float64))
	if iat < before || iat > after {
		t.Fatalf("iat = %d; want between %d and %d", iat, before, after)
	}
}
