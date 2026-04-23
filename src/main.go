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

func main() {
	mux := http.NewServeMux()
	logger := &utils.Logger{Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(),
	}))}

	portStr := os.Getenv("API_PORT")
	if portStr == "" {
		portStr = "3000"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		port = 3000
	}

	client := &http.Client{Timeout: time.Second * 5}
	log.Println("Initialising DB")
	db, err := db.InitialiseDB("./test.sqlite", "seed_profiles.json")
	log.Println("DB initialised successfully :)")

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
	mux.HandleFunc("GET /api/profiles/search", p.SearchProfiles)
	mux.HandleFunc("GET /api/profiles", p.GetProfiles)
	mux.HandleFunc("DELETE /api/profiles/{id}", p.DeleteProfile)

	app := requestLoggerMiddleware(logger, corsMiddleware(mux))
	app = reqIdMiddleware(recoverPanicMiddleware(app))

	log.Printf("server starting on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), app); err != nil {
		log.Fatalf("server error: %v\n", err)
	}
}
