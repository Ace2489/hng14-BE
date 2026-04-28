package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hng-s1/src/utils"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type AuthHandler struct {
	redis  *redis.Client
	gh     *utils.GithubOauth
	client *http.Client
}

func (h *AuthHandler) GithubLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := utils.LoggerFromCtx(ctx)
	port := r.URL.Query().Get("port")

	logger.Info("Generating oauth 'state'")
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		logger.Error("Failed to generate random 'state'", "error:", err)
		utils.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	state := base64.RawURLEncoding.EncodeToString(b)

	logger.Info("Storing generated oauth 'state'")

	if err := h.redis.Set(r.Context(), "state_"+state, port, 10*time.Minute).Err(); err != nil {
		logger.Info("Error storing generated oauth secret", "error:", err)
		utils.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	p := url.Values{
		"redirect_uri": {h.gh.RedirectUri},
		"client_id":    {h.gh.ClientId},
		"scope":        {"read:user user:email"},
		"state":        {state},
	}

	logger.InfoFmt("Redirecting to %s to complete authentication", h.gh.RedirectUri)
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+p.Encode(), http.StatusTemporaryRedirect)
}

func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	ctx := r.Context()

	logger := utils.LoggerFromCtx(ctx)

	logger.Info("Checking for oauth 'state' in cached values")

	port, err := h.redis.GetDel(r.Context(), "state_"+state).Result()
	if err != nil {
		logger.Error("Redis state retrieval failed", "state", state, "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "An unexpected error occurred")
		return
	}

	body := url.Values{
		"client_id":     {h.gh.ClientId},
		"client_secret": {h.gh.ClientSecret},
		"code":          {code},
		"redirect_uri":  {h.gh.RedirectUri},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token",
		strings.NewReader(body.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	logger.Info("Obtaining tokens from GitHub")
	resp, err := h.client.Do(req)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "Token exchange failed")
		return
	}
	defer resp.Body.Close()

	var t struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	logger.Info("Decoding token from GitHub")
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		logger.Error("Failed to decode GitHub's response", "error:", err)
		utils.WriteError(w, http.StatusInternalServerError, "Failed to decode the Oauth server's response")
		return
	}

	if t.Error != "" {
		logger.Error("GitHub sent an error instead of a token", "error:", err)
		utils.WriteError(w, http.StatusBadRequest, fmt.Sprintf("github: %s", t.ErrorDesc))
		return
	}

	if port != "" {
		http.Redirect(w, r, fmt.Sprintf("http://localhost:%s/callback?token=%s", port, t.AccessToken), http.StatusTemporaryRedirect)
		return
	}
	utils.WriteSuccess(w, http.StatusOK, t.AccessToken)
}
