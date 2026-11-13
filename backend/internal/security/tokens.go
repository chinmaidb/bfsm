package security

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenIssuer struct {
	privateKey *rsa.PrivateKey
	issuer     string
	lifetime   time.Duration
}

type AccessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func LoadTokenIssuer(privateKeyPath, issuer string, lifetime time.Duration) (*TokenIssuer, error) {
	keyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read token signing key: %w", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse token signing key: %w", err)
	}
	if lifetime <= 0 {
		return nil, fmt.Errorf("token lifetime must be positive")
	}
	return &TokenIssuer{privateKey: key, issuer: issuer, lifetime: lifetime}, nil
}

func (i *TokenIssuer) Issue(userID, role string) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: i.issuer, Subject: userID,
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(i.lifetime)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(i.privateKey)
}

func (i *TokenIssuer) Verify(token string) (*AccessClaims, error) {
	return ParseAccessToken(token, &i.privateKey.PublicKey, i.issuer)
}

// ParseAccessToken restricts accepted algorithms to RSA SHA-256. Load the
// verification key from the matching public key file in the API composition root.
func ParseAccessToken(token string, publicKey *rsa.PublicKey, issuer string) (*AccessClaims, error) {
	claims := new(AccessClaims)
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodRS256 {
			return nil, fmt.Errorf("unexpected token algorithm")
		}
		return publicKey, nil
	}, jwt.WithIssuer(issuer), jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("invalid access token")
	}
	return claims, nil
}
