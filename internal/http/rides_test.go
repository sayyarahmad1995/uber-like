package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	rideapp "github.com/sayyarahmad1995/uber-like/internal/application/ride"
	domainride "github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

type createRideRepository struct {
	created domainride.Ride
}

func (r *createRideRepository) Get(context.Context, domainride.ID) (domainride.Ride, error) {
	return domainride.Ride{}, application.ErrNotFound
}

func (r *createRideRepository) Create(_ context.Context, ride domainride.Ride) error {
	r.created = ride
	return nil
}

func (r *createRideRepository) Save(context.Context, domainride.Ride) error {
	return nil
}

func TestCreateRideHandlerRequiresIdentity(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides",
		nil,
	)
	rec := httptest.NewRecorder()

	h := CreateRideHandler{
		CreateRide: rideapp.CreateRide{},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateRideHandlerCreatesRideForAuthenticatedUser(t *testing.T) {
	riderID := uuid.New()
	repo := &createRideRepository{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides",
		strings.NewReader(`{
			"pickup": {
				"latitude": 24.8607,
				"longitude": 67.0011
			},
			"dropoff": {
				"latitude": 24.8615,
				"longitude": 67.0099
			}
		}`),
	)

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "kratos-subject",
			UserID:  riderID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := CreateRideHandler{
		CreateRide: rideapp.CreateRide{Rides: repo},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if repo.created.RiderID != riderID {
		t.Fatalf("rider_id = %s, want %s", repo.created.RiderID, riderID)
	}

	if repo.created.Status != domainride.StatusRequested {
		t.Fatalf("status = %s, want %s", repo.created.Status, domainride.StatusRequested)
	}

	if repo.created.Pickup.Latitude != 24.8607 ||
		repo.created.Pickup.Longitude != 67.0011 {
		t.Fatalf("unexpected pickup: %#v", repo.created.Pickup)
	}

	if repo.created.Dropoff.Latitude != 24.8615 ||
		repo.created.Dropoff.Longitude != 67.0099 {
		t.Fatalf("unexpected dropoff: %#v", repo.created.Dropoff)
	}

	var response domainride.Ride
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID != repo.created.ID {
		t.Fatalf("response ID = %s, want %s", response.ID, repo.created.ID)
	}
}

func TestCreateRideHandlerRejectsInvalidCoordinates(t *testing.T) {
	riderID := uuid.New()
	repo := &createRideRepository{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides",
		strings.NewReader(`{
			"pickup": {
				"latitude": 100,
				"longitude": 67.0011
			},
			"dropoff": {
				"latitude": 24.8615,
				"longitude": 67.0099
			}
		}`),
	)

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "kratos-subject",
			UserID:  riderID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := CreateRideHandler{
		CreateRide: rideapp.CreateRide{Rides: repo},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateRideHandlerRejectsUnknownFields(t *testing.T) {
	riderID := uuid.New()
	repo := &createRideRepository{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides",
		strings.NewReader(`{
			"rider_id": "11111111-1111-1111-1111-111111111111",
			"pickup": {
				"latitude": 24.8607,
				"longitude": 67.0011
			},
			"dropoff": {
				"latitude": 24.8615,
				"longitude": 67.0099
			}
		}`),
	)

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "kratos-subject",
			UserID:  riderID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := CreateRideHandler{
		CreateRide: rideapp.CreateRide{Rides: repo},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateRideHandlerRejectsMissingCoordinates(t *testing.T) {
	riderID := uuid.New()
	repo := &createRideRepository{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides",
		strings.NewReader(`{}`),
	)

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "kratos-subject",
			UserID:  riderID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := CreateRideHandler{
		CreateRide: rideapp.CreateRide{Rides: repo},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateRideHandlerRejectsMissingDropoff(t *testing.T) {
	riderID := uuid.New()
	repo := &createRideRepository{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides",
		strings.NewReader(`{
			"pickup": {
				"latitude": 24.8607,
				"longitude": 67.0011
			}
		}`),
	)

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "kratos-subject",
			UserID:  riderID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := CreateRideHandler{
		CreateRide: rideapp.CreateRide{Rides: repo},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

type startBiddingRepository struct {
	ride  domainride.Ride
	saved domainride.Ride
}

func (r *startBiddingRepository) Get(_ context.Context, id domainride.ID) (domainride.Ride, error) {
	if r.ride.ID == uuid.Nil || r.ride.ID != id {
		return domainride.Ride{}, application.ErrNotFound
	}

	return r.ride, nil
}

func (r *startBiddingRepository) Create(context.Context, domainride.Ride) error {
	return nil
}

func (r *startBiddingRepository) Save(_ context.Context, ride domainride.Ride) error {
	r.saved = ride
	return nil
}

func TestStartBiddingHandlerRequiresIdentity(t *testing.T) {
	rideID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides/"+rideID.String()+"/bidding/start",
		nil,
	)
	req.SetPathValue("rideID", rideID.String())

	rec := httptest.NewRecorder()

	h := StartBiddingHandler{
		StartBidding: rideapp.StartBidding{},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStartBiddingHandlerStartsBiddingForRideOwner(t *testing.T) {
	riderID := uuid.New()
	rideID := uuid.New()

	ride, err := domainride.New(
		rideID,
		riderID,
		domainride.Coordinate{
			Latitude:  24.8607,
			Longitude: 67.0011,
		},
		domainride.Coordinate{
			Latitude:  24.8615,
			Longitude: 67.0099,
		},
	)
	if err != nil {
		t.Fatalf("create ride: %v", err)
	}

	repo := &startBiddingRepository{
		ride: ride,
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides/"+rideID.String()+"/bidding/start",
		nil,
	)
	req.SetPathValue("rideID", rideID.String())

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "kratos-subject",
			UserID:  riderID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := StartBiddingHandler{
		StartBidding: rideapp.StartBidding{
			Rides: repo,
		},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if repo.saved.Status != domainride.StatusBidding {
		t.Fatalf("saved status = %s, want %s", repo.saved.Status, domainride.StatusBidding)
	}

	var response domainride.Ride
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID != rideID {
		t.Fatalf("response ID = %s, want %s", response.ID, rideID)
	}

	if response.Status != domainride.StatusBidding {
		t.Fatalf("response status = %s, want %s", response.Status, domainride.StatusBidding)
	}
}

func TestStartBiddingHandlerRejectsNonOwner(t *testing.T) {
	riderID := uuid.New()
	otherUserID := uuid.New()
	rideID := uuid.New()

	ride, err := domainride.New(
		rideID,
		riderID,
		domainride.Coordinate{
			Latitude:  24.8607,
			Longitude: 67.0011,
		},
		domainride.Coordinate{
			Latitude:  24.8615,
			Longitude: 67.0099,
		},
	)
	if err != nil {
		t.Fatalf("create ride: %v", err)
	}

	repo := &startBiddingRepository{
		ride: ride,
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides/"+rideID.String()+"/bidding/start",
		nil,
	)
	req.SetPathValue("rideID", rideID.String())

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "other-subject",
			UserID:  otherUserID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := StartBiddingHandler{
		StartBidding: rideapp.StartBidding{
			Rides: repo,
		},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	if repo.saved.ID != uuid.Nil {
		t.Fatalf("ride was unexpectedly saved")
	}
}

func TestStartBiddingHandlerRejectsInvalidTransition(t *testing.T) {
	riderID := uuid.New()
	rideID := uuid.New()

	ride, err := domainride.New(
		rideID,
		riderID,
		domainride.Coordinate{
			Latitude:  24.8607,
			Longitude: 67.0011,
		},
		domainride.Coordinate{
			Latitude:  24.8615,
			Longitude: 67.0099,
		},
	)
	if err != nil {
		t.Fatalf("create ride: %v", err)
	}

	if err := ride.StartBidding(); err != nil {
		t.Fatalf("start bidding: %v", err)
	}

	repo := &startBiddingRepository{
		ride: ride,
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides/"+rideID.String()+"/bidding/start",
		nil,
	)
	req.SetPathValue("rideID", rideID.String())

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "kratos-subject",
			UserID:  riderID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := StartBiddingHandler{
		StartBidding: rideapp.StartBidding{
			Rides: repo,
		},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}

	if repo.saved.ID != uuid.Nil {
		t.Fatalf("ride was unexpectedly saved")
	}
}

func TestStartBiddingHandlerRejectsInvalidRideID(t *testing.T) {
	riderID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides/not-a-uuid/bidding/start",
		nil,
	)
	req.SetPathValue("rideID", "not-a-uuid")

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "kratos-subject",
			UserID:  riderID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := StartBiddingHandler{
		StartBidding: rideapp.StartBidding{},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartBiddingHandlerReturnsNotFoundForUnknownRide(t *testing.T) {
	riderID := uuid.New()
	rideID := uuid.New()

	repo := &startBiddingRepository{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/rides/"+rideID.String()+"/bidding/start",
		nil,
	)
	req.SetPathValue("rideID", rideID.String())

	ctx := context.WithValue(
		req.Context(),
		identityContextKey{},
		application.Identity{
			Subject: "kratos-subject",
			UserID:  riderID,
		},
	)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h := StartBiddingHandler{
		StartBidding: rideapp.StartBidding{
			Rides: repo,
		},
	}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
