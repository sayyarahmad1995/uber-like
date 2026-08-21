package kratos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sayyarahmad1995/uber-like/internal/application"
	"github.com/sayyarahmad1995/uber-like/internal/domain/user"
)

var (
	ErrInvalidSession = errors.New("invalid Kratos session")
	ErrUnknownUser    = errors.New("authenticated user not found")
	ErrInactiveUser   = errors.New("authenticated user is inactive")
)

type UserRepository interface {
	GetByOIDCSubject(ctx context.Context, subject string) (user.User, error)
}

type Resolver struct {
	BaseURL string
	Client  *http.Client
	Users   UserRepository
}

type whoAmIResponse struct {
	Active   bool `json:"active"`
	Identity struct {
		ID string `json:"id"`
	} `json:"identity"`
}

func NewResolver(baseURL string, users UserRepository) *Resolver {
	return &Resolver{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
		Users: users,
	}
}

func (r *Resolver) Resolve(ctx context.Context, token string) (application.Identity, error) {
	if r == nil || r.Users == nil || r.Client == nil || strings.TrimSpace(r.BaseURL) == "" {
		return application.Identity{}, ErrInvalidSession
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return application.Identity{}, ErrInvalidSession
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.BaseURL+"/sessions/whoami", nil)
	if err != nil {
		return application.Identity{}, fmt.Errorf("create Kratos whoami request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := r.Client.Do(req)
	if err != nil {
		return application.Identity{}, fmt.Errorf("call Kratos whoami: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, res.Body)
		return application.Identity{}, ErrInvalidSession
	}

	var payload whoAmIResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return application.Identity{}, fmt.Errorf("decode Kratos whoami response: %w", err)
	}
	if !payload.Active || strings.TrimSpace(payload.Identity.ID) == "" {
		return application.Identity{}, ErrInvalidSession
	}

	subject := strings.TrimSpace(payload.Identity.ID)
	localUser, err := r.Users.GetByOIDCSubject(ctx, subject)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			return application.Identity{}, ErrUnknownUser
		}
		return application.Identity{}, fmt.Errorf("resolve local user: %w", err)
	}
	if !localUser.IsActive() {
		return application.Identity{}, ErrInactiveUser
	}

	return application.Identity{
		Subject: subject,
		UserID:  localUser.ID,
	}, nil
}

var _ application.IdentityResolver = (*Resolver)(nil)
