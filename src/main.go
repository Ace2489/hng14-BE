package main

import (
	"fmt"
	"hng-s1/src/data"
	"hng-s1/src/handlers"
	"hng-s1/src/utils"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func logLevel() slog.Level {
	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func main() {
	logger := &utils.Logger{Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(),
	}))}

	cfg := utils.Config{}

	if err := cfg.Load(); err != nil {
		log.Fatalf("Failed to initialise environment. %v", err)
	}

	client := &http.Client{Timeout: time.Second * 5}

	log.Println("Initialising DB")
	db, err := data.InitialiseDB("./test.sqlite", "seed_profiles.json")
	if err != nil {
		log.Fatalf("failed to initialise DB: %v", err)
	}
	log.Println("DB initialised successfully :)")

	deps := handlers.Dependencies{
		DB:        db,
		Client:    client,
		Redis:     cfg.Redis,
		Gh:        cfg.Gh,
		JwtSecret: cfg.JwtSecret,
	}

	mux := http.NewServeMux()
	v1 := http.NewServeMux()
	auth := AuthMiddleware{JwtSecret: cfg.JwtSecret}
	adminGuard := requireRole(data.RoleAdmin)
	rateLimiter := RateLimiter{Redis: cfg.Redis}

	var g = deps.GenderizeHandler()
	var p = deps.ProfileHandler()
	var a = deps.AuthHandler()

	// public — no auth
	v1.HandleFunc("GET /auth/github", a.GitHubLogin)
	v1.HandleFunc("GET /auth/github/callback", a.GitHubCallback)
	v1.HandleFunc("POST /auth/refresh", a.HandleRefresh)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /classify", g.HandleGender)

	protected.HandleFunc("POST /profiles", p.CreateProfile)
	protected.HandleFunc("GET /profiles/{id}", adminGuard(p.GetProfile))
	protected.HandleFunc("GET /profiles/search", p.SearchProfiles)
	protected.HandleFunc("GET /profiles", p.GetProfiles)
	protected.HandleFunc("DELETE /profiles/{id}", adminGuard(p.DeleteProfile))
	protected.Handle("GET /profiles/export", adminGuard(p.ExportCSV))

	protected.HandleFunc("GET /auth/me", a.HandleMe)

	v1.Handle("/", auth.Guard(protected))
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	app := requestLoggerMiddleware(logger, rateLimiter.Middleware(corsMiddleware(mux)))
	app = reqIdMiddleware(recoverPanicMiddleware(app))

	serverUrl := fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port)
	log.Printf("server starting on %s", serverUrl)
	if err := http.ListenAndServe(serverUrl, app); err != nil {
		log.Fatalf("server error: %v\n", err)
	}
}
