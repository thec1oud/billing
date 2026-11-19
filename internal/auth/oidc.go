package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
)

type Principal struct {
	Subject string
	Name    string
}

type contextKey struct{}

type Authenticator interface {
	Authenticate(token string) (Principal, error)
}

type OIDCVerifier struct {
	keys     keyfunc.Keyfunc
	issuer   string
	audience string
}

func NewOIDCVerifier(ctx context.Context, jwksURL, issuer, audience string) (*OIDCVerifier, error) {
	if jwksURL == "" || issuer == "" || audience == "" {
		return nil, errors.New("AUTH_JWKS_URL, AUTH_ISSUER, and AUTH_AUDIENCE are required")
	}
	keys, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("load OIDC signing keys: %w", err)
	}
	return &OIDCVerifier{keys: keys, issuer: issuer, audience: audience}, nil
}

type accessTokenClaims struct {
	Name string `json:"name,omitempty"`
	jwt.RegisteredClaims
}

func (v *OIDCVerifier) Authenticate(raw string) (Principal, error) {
	if v == nil || v.keys == nil {
		return Principal{}, errors.New("OIDC verifier unavailable")
	}
	var claims accessTokenClaims
	token, err := jwt.ParseWithClaims(
		raw,
		&claims,
		v.keys.Keyfunc,
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	)
	if err != nil || token == nil || !token.Valid {
		return Principal{}, errors.New("invalid access token")
	}
	if claims.Subject == "" || claims.ExpiresAt == nil {
		return Principal{}, errors.New("access token requires sub and exp claims")
	}
	return Principal{Subject: claims.Subject, Name: claims.Name}, nil
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(contextKey{}).(Principal)
	return principal, ok
}

func ActorFromContext(ctx context.Context, fallbackID string) eventmodel.Actor {
	if principal, ok := PrincipalFromContext(ctx); ok {
		return eventmodel.Actor{
			Type: eventmodel.ActorTypeUser,
			ID:   principal.Subject,
			Name: principal.Name,
		}
	}
	return eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: fallbackID}
}

func BearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}
