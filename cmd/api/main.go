package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/ivantung/todo-backend/internal/auth"
	"github.com/ivantung/todo-backend/internal/config"
	"github.com/ivantung/todo-backend/internal/db"
	"github.com/ivantung/todo-backend/internal/handler"
	appmw "github.com/ivantung/todo-backend/internal/middleware"
	"github.com/ivantung/todo-backend/internal/oauth"
	"github.com/ivantung/todo-backend/internal/repository"
	"github.com/ivantung/todo-backend/internal/service"
)

func main() {
	cfg := config.Load()

	conn, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := db.Migrate(ctx, conn, cfg.MigrationsDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	signer := auth.NewSigner(cfg.JWTSecret, cfg.JWTTTL)

	userRepo := repository.NewUserRepo(conn)
	columnRepo := repository.NewColumnRepo(conn)
	cardRepo := repository.NewCardRepo(conn)

	authSvc := service.NewAuthService(userRepo, signer)
	boardSvc := service.NewBoardService(columnRepo, cardRepo)
	columnSvc := service.NewColumnService(columnRepo)
	cardSvc := service.NewCardService(cardRepo, columnRepo)

	authH := handler.NewAuthHandler(authSvc, userRepo)
	boardH := handler.NewBoardHandler(boardSvc)
	columnH := handler.NewColumnHandler(columnSvc)
	cardH := handler.NewCardHandler(cardSvc)

	var oauthH *handler.OAuthHandler
	if cfg.OAuthEnabled() {
		googleSvc := oauth.NewService(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
		oauthH = handler.NewOAuthHandler(googleSvc, authSvc, cfg.FrontendURL)
		log.Printf("google oauth enabled (redirect=%s)", cfg.GoogleRedirectURL)
	} else {
		log.Printf("google oauth disabled — set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET to enable")
	}

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		ExposedHeaders:   []string{},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)

		if oauthH != nil {
			r.Get("/auth/google/start", oauthH.Start)
			r.Get("/auth/google/callback", oauthH.Callback)
		}

		r.Group(func(r chi.Router) {
			r.Use(appmw.RequireAuth(signer))
			r.Post("/auth/logout", authH.Logout)
			r.Get("/auth/me", authH.Me)

			// Board
			r.Get("/board", boardH.Get)

			// Columns — register /reorder BEFORE /{id} so chi doesn't capture "reorder" as id
			r.Post("/columns", columnH.Create)
			r.Put("/columns/reorder", columnH.Reorder)
			r.Put("/columns/{id}", columnH.Update)
			r.Delete("/columns/{id}", columnH.Delete)

			// Cards
			r.Post("/cards", cardH.Create)
			r.Get("/cards/{id}", cardH.Get)
			r.Put("/cards/{id}", cardH.Update)
			r.Delete("/cards/{id}", cardH.Delete)
			r.Put("/cards/{id}/labels", cardH.SetLabels)
		})
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
