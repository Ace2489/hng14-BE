package main

import (
	"context"
	"fmt"
	"hng-s1/src/data"
	"hng-s1/src/utils"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/redis/go-redis/v9"
)

// Thin wrapper to catch the response code after being written
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
				stack := debug.Stack()
				log.Printf("panic: %v\n%s", rec, stack)
				utils.WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)

	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := utils.LoggerFromCtx(r.Context())

		// w.Header().Set("Access-Control-Allow-Origin", "SameSite")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		logger.Debug("Writing CORS headers for OPTION request")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func requestLoggerMiddleware(logger *utils.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := IdFromCtx(r.Context())

		log := utils.Logger{Logger: logger.With("request_id", reqId, "method", r.Method, "path", r.URL.Path)}
		log.Info("Request received")

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		ctx := context.WithValue(r.Context(), utils.LoggerKey{}, log)

		next.ServeHTTP(rw, r.WithContext(ctx))

		log.Info("Response sent", "status", rw.status)
	})
}

type AuthMiddleware struct{ JwtSecret string }

func (a *AuthMiddleware) Guard(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := utils.LoggerFromCtx(r.Context())

		logger.Info("Auth guard checking request")
		cookie, err := r.Cookie("access_token")
		if err != nil {
			logger.Error("No access token found in the request cookie")
			utils.WriteError(w, http.StatusUnauthorized, "missing token")
			return
		}
		claims, err := utils.ParseAccessToken(cookie.Value, []byte(a.JwtSecret))
		if err != nil {
			logger.Error("Invalid access token found in the request cookie")
			utils.WriteError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		logger.Info("Auth guard successfully validated request. Continuing with the chain")
		ctx := context.WithValue(r.Context(), utils.ClaimsKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

type RateLimiter struct{ Redis *redis.Client }

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		key := "rl_" + ip

		count, err := l.Redis.Incr(r.Context(), key).Result()
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if count == 1 {
			l.Redis.Expire(r.Context(), key, time.Minute)
		}
		if count > 100 {
			utils.LoggerFromCtx(r.Context()).Error("rate limit exceeded", "ip", ip, "count", count)
			utils.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}
func requireRole(role data.Role) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			logger := utils.LoggerFromCtx(r.Context())

			logger.InfoFmt("Role guard running for role: %s", role)
			claims, ok := utils.ClaimsFromCtx(r.Context())
			if !ok || claims.Role != role {
				logger.Error("Request does not have the sufficient permissions.", "permission needed", role)
				utils.WriteError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			logger.Info("Role validation successful. Continuing  with request chain")
			next.ServeHTTP(w, r)
		}
	}
}

func IdFromCtx(ctx context.Context) string {
	if req, ok := ctx.Value(RequestId{}).(string); ok && req != "" {
		return req
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
