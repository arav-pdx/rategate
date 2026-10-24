package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Authenticator struct {
	publicKey *rsa.PublicKey
	tierClaim string
}

type Identity struct {
	Subject string
	Tier    string
}

func NewAuthenticator(pubKeyPath, tierClaim string) (*Authenticator, error) {
	keyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading public key: %w", err)
	}
	rsaPub, err := jwt.ParseRSAPublicKeyFromPEM(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing rsa public key: %w", err)
	}

	if tierClaim == "" {
		tierClaim = "tier"
	}

	return &Authenticator{
		publicKey: rsaPub,
		tierClaim: tierClaim,
	}, nil
}

func (a *Authenticator) Verify(authHeader string) (Identity, error) {
	if authHeader == "" {
		return Identity{}, errors.New("missing authorization header")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return Identity{}, errors.New("invalid authorization token format")
	}

	tokenStr := parts[1]
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.publicKey, nil
	})

	if err != nil || !token.Valid {
		return Identity{}, errors.New("invalid or expired token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Identity{}, errors.New("malformed claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return Identity{}, errors.New("sub claim required")
	}

	tier, _ := claims[a.tierClaim].(string)
	if tier == "" {
		tier = "free"
	}

	return Identity{Subject: sub, Tier: tier}, nil
}
