package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
)

type identityContextKey struct{}

type AuthMiddleware struct {
	Resolver application.IdentityResolver
}

func BearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" || len(header) < 7 || !strings.EqualFold(header[:7], "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(header[7:])
	return token, token != ""
}

func (m AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.Resolver == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, ok := BearerToken(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		identity, err := m.Resolver.Resolve(r.Context(), token)
		if err != nil || identity.UserID == uuid.Nil || identity.Subject == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), identityContextKey{}, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func IdentityFromContext(ctx context.Context) (application.Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(application.Identity)
	return identity, ok
}

var ErrMissingIdentity = errors.New("authenticated identity missing from context")

func MustIdentity(ctx context.Context) (application.Identity, error) {
	identity, ok := IdentityFromContext(ctx)
	if !ok {
		return application.Identity{}, ErrMissingIdentity
	}
	return identity, nil
}
