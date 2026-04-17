package main

import (
	"database/sql"
	"fmt"
	"hng-s1/src/handlers"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
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

func bootstrap(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS profiles (
			id                  TEXT PRIMARY KEY,
			name                TEXT NOT NULL,
			gender              TEXT NOT NULL,
			gender_probability  REAL NOT NULL,
			sample_size         INTEGER NOT NULL,
			age                 INTEGER NOT NULL,
			age_group           TEXT NOT NULL,
			country_id          TEXT NOT NULL,
			country_probability REAL NOT NULL,
			created_at          TEXT NOT NULL
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_name_lower ON profiles (LOWER(name));
	`)
	return err
}
func main() {
	mux := http.NewServeMux()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(),
	}))

	portStr := os.Getenv("API_PORT")
	if portStr == "" {
		portStr = "3000"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		port = 3000
	}

	client := &http.Client{Timeout: time.Second * 5}
	db, err := sql.Open("sqlite3", "./test.sqlite")
	if err != nil {
		log.Fatalf("DB failed to open: %s\n", err)
	}
	if err := bootstrap(db); err != nil {
		log.Fatalf("DB bootstrap failed: %s\n", err)
	}

	deps := handlers.Dependencies{
		DB:     db,
		Client: client,
	}

	var g = deps.GenderizeHandler()
	mux.HandleFunc("/", handlers.HandleNotFound)
	mux.HandleFunc("GET /api/classify", g.HandleGender)

	var p = deps.ProfileHandler()
	mux.HandleFunc("POST /api/profiles", p.CreateProfile)
	mux.HandleFunc("GET /api/profiles/{id}", p.GetProfile)
	mux.HandleFunc("GET /api/profiles", p.GetProfiles)
	mux.HandleFunc("DELETE /api/profiles/{id}", p.DeleteProfile)

	app := requestLoggerMiddleware(logger, corsMiddleware(mux))
	app = reqIdMiddleware(recoverPanicMiddleware(app))

	log.Printf("server starting on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), app); err != nil {
		log.Fatalf("server error: %v\n", err)
	}
}
