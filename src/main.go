package main

import (
	"fmt"
	"hng-s1/src/db"
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
	db, err := db.InitialiseDB("./test.sqlite", "seed_profiles.json")
	if err != nil {
		log.Fatalf("failed to initialise DB: %v", err)
	}
	log.Println("DB initialised successfully :)")

	deps := handlers.Dependencies{
		DB:     db,
		Client: client,
		Redis:  cfg.Redis,
		Gh:     cfg.Gh,
	}

	mux := http.NewServeMux()
	v1 := http.NewServeMux()

	var g = deps.GenderizeHandler()
	v1.HandleFunc("/", handlers.HandleNotFound)
	v1.HandleFunc("GET /classify", g.HandleGender)

	var p = deps.ProfileHandler()
	v1.HandleFunc("POST /profiles", p.CreateProfile)
	v1.HandleFunc("GET /profiles/{id}", p.GetProfile)
	v1.HandleFunc("GET /profiles/search", p.SearchProfiles)
	v1.HandleFunc("GET /profiles", p.GetProfiles)
	v1.HandleFunc("DELETE /profiles/{id}", p.DeleteProfile)

	var a = deps.AuthHandler()
	v1.HandleFunc("GET /auth/github", a.GithubLogin)
	v1.HandleFunc("GET /auth/github/callback", a.GitHubCallback)
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	app := requestLoggerMiddleware(logger, corsMiddleware(mux))
	app = reqIdMiddleware(recoverPanicMiddleware(app))

	serverUrl := fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port)
	log.Printf("server starting on %s", serverUrl)
	if err := http.ListenAndServe(serverUrl, app); err != nil {
		log.Fatalf("server error: %v\n", err)
	}
}
