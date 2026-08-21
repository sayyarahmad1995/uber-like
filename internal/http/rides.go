package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/sayyarahmad1995/uber-like/internal/application"
	rideapp "github.com/sayyarahmad1995/uber-like/internal/application/ride"
	domainride "github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

type CreateRideHandler struct {
	CreateRide rideapp.CreateRide
}

type createRideRequest struct {
	Pickup  coordinateRequest `json:"pickup"`
	Dropoff coordinateRequest `json:"dropoff"`
}

type createRideResponse struct {
	ID      string            `json:"id"`
	RiderID string            `json:"rider_id"`
	Status  domainride.Status `json:"status"`
	Pickup  coordinateRequest `json:"pickup"`
	Dropoff coordinateRequest `json:"dropoff"`
}

type coordinateRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func (h CreateRideHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := MustIdentity(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req createRideRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ride, err := h.CreateRide.Execute(
		r.Context(),
		identity.UserID,
		domainride.Coordinate{
			Latitude:  req.Pickup.Latitude,
			Longitude: req.Pickup.Longitude,
		},
		domainride.Coordinate{
			Latitude:  req.Dropoff.Latitude,
			Longitude: req.Dropoff.Longitude,
		},
	)
	if err != nil {
		switch err {
		case application.ErrInvalidArgument:
			http.Error(w, "invalid request", http.StatusBadRequest)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	response := createRideResponse{
		ID:      ride.ID.String(),
		RiderID: ride.RiderID.String(),
		Status:  ride.Status,
		Pickup: coordinateRequest{
			Latitude:  ride.Pickup.Latitude,
			Longitude: ride.Pickup.Longitude,
		},
		Dropoff: coordinateRequest{
			Latitude:  ride.Dropoff.Latitude,
			Longitude: ride.Dropoff.Longitude,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}
