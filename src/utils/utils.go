package utils

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
)

type Payload struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type LoggerKey struct{}

func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") //REDUNDANT: This is now handled by the middleware
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, Payload{Status: "error", Message: message})
}

func WriteSuccess(w http.ResponseWriter, status int, data interface{}) {
	WriteSuccessWithMessage(w, status, "", data)
}
func WriteSuccessWithMessage(w http.ResponseWriter, status int, message string, data interface{}) {
	WriteJSON(w, status, Payload{Status: "success", Message: message, Data: data})
}

func WriteList(w http.ResponseWriter, status int, count int, data interface{}) {
	type listPayload struct {
		Status string      `json:"status"`
		Count  int         `json:"count"`
		Data   interface{} `json:"data"`
	}
	WriteJSON(w, status, listPayload{Status: "success", Count: count, Data: data})
}

func LoggerFromCtx(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(LoggerKey{}).(*slog.Logger); ok {
		return logger
	}
	log.Println("No logger found. Falling back the default logger")
	return slog.Default()
}
