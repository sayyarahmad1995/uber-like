package httpapi

import (
	"net/http"

	"github.com/sayyarahmad1995/uber-like/internal/application"
)

type LogoutHandler struct {
	Auth application.AuthService
}

func (h LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token, ok := BearerToken(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.Auth.Logout(r.Context(), token); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
