package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	rideapp "github.com/sayyarahmad1995/uber-like/internal/application/ride"
	httpapi "github.com/sayyarahmad1995/uber-like/internal/http"
	"github.com/sayyarahmad1995/uber-like/internal/infrastructure/kratos"
	"github.com/sayyarahmad1995/uber-like/internal/infrastructure/postgres"
	"github.com/sayyarahmad1995/uber-like/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		log.Printf("api: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	store := postgres.New(db)
	identityResolver := kratos.NewResolver(cfg.KratosPublicURL, store.Users())
	authService := application.AuthService{Sessions: identityResolver, Users: store.Users()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ping(r.Context()); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})
	mux.Handle("POST /api/v1/auth/provision", httpapi.ProvisionHandler{Auth: authService})
	mux.Handle("POST /api/v1/auth/logout", httpapi.LogoutHandler{Auth: authService})

	protected := httpapi.AuthMiddleware{Resolver: identityResolver}.Middleware

	mux.Handle(
		"GET /api/v1/auth/session",
		protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, err := httpapi.MustIdentity(r.Context())
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user_id":"` + identity.UserID.String() + `","subject":"` + identity.Subject + `"}`))
		})),
	)

	mux.Handle(
		"POST /api/v1/rides",
		protected(httpapi.CreateRideHandler{
			CreateRide: rideapp.CreateRide{
				Rides: store.Rides(),
			},
		}),
	)

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
