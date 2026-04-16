package main

import (
	"fmt"
	"hng-s1/src/handlers"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
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

	client := &http.Client{Timeout: time.Second * 3}
	deps := handlers.Dependencies{Client: client}

	mux.HandleFunc("/", handlers.HandleNotFound)

	var g = deps.GenderizeHandler()
	mux.HandleFunc("GET /api/classify", g.HandleGender)

	app := requestLoggerMiddleware(logger, mux)
	app = recoverPanicMiddleware(corsMiddleware(app))

	logger.Info("server starting", "port", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), app); err != nil {
		fmt.Printf("server error: %v\n", err)
		os.Exit(1)
	}
}
