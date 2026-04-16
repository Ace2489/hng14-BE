package main

import (
	"context"
	"fmt"
	"hng-s1/src/utils"
	"log"
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

type RequestId struct{}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func reqIdMiddleware(next http.Handler) http.Handler {
	//This is meant to be the first one in the chain
	//If this method panics, the results will very likely leak to the client
	//Or have some kind of "closed connection" error
	//Due to that, this has to be as light as possible and avoid panics at all costs
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := IdFromCtx(r.Context())
		ctx := context.WithValue(r.Context(), RequestId{}, reqId)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func recoverPanicMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqId := IdFromCtx(r.Context())
				log.Printf("Server panicked with error: %s. request_id: %s\n", rec, reqId)
				utils.WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)

	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := utils.LoggerFromCtx(r.Context())

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		fmt.Printf("CORS: %s\n", logger)
		logger.Debug("Writing CORS headers for OPTION request")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func requestLoggerMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := IdFromCtx(r.Context())

		log := logger.With("request_id", reqId, "method", r.Method, "path", r.URL.Path)
		log.Info("Request received")

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		ctx := context.WithValue(r.Context(), utils.LoggerKey{}, log)

		next.ServeHTTP(rw, r.WithContext(ctx))

		log.Info("Response sent", "status", rw.status)
	})
}

func IdFromCtx(ctx context.Context) string {
	if req, ok := ctx.Value(RequestId{}).(string); ok && req != "" {
		return req
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
