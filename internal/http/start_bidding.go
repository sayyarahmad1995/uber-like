package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	rideapp "github.com/sayyarahmad1995/uber-like/internal/application/ride"
	domainride "github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

type StartBiddingHandler struct {
	StartBidding rideapp.StartBidding
}

func (h StartBiddingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := MustIdentity(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rideID, err := uuid.Parse(r.PathValue("rideID"))
	if err != nil {
		http.Error(w, "invalid ride id", http.StatusBadRequest)
		return
	}

	ride, err := h.StartBidding.Execute(
		r.Context(),
		domainride.ID(rideID),
		identity.UserID,
	)
	if err != nil {
		switch err {
		case application.ErrNotFound:
			http.Error(w, "not found", http.StatusNotFound)
		case application.ErrForbidden:
			http.Error(w, "forbidden", http.StatusForbidden)
		case domainride.ErrInvalidTransition:
			http.Error(w, "invalid ride state", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ride)
}
