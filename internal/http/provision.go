package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/sayyarahmad1995/uber-like/internal/application"
)

type ProvisionHandler struct {
	Auth application.AuthService
}

func (h ProvisionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token, ok := BearerToken(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	identity, err := h.Auth.Provision(r.Context(), token)
	if err != nil {
		if err == application.ErrInvalidArgument {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(identity)
}
