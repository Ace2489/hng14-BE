package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"hng-s1/src/data"
	"hng-s1/src/utils"

	"github.com/redis/go-redis/v9"
)

type AuthHandler struct {
	redis     *redis.Client
	gh        *utils.GithubOauth
	db        *sql.DB
	client    *http.Client
	jwtSecret string
	userRepo  *data.UserRepo
}

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

func (h *AuthHandler) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomBase64(16)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	port := r.URL.Query().Get("port")
	if err := h.redis.Set(r.Context(), "state_"+state, port, 10*time.Minute).Err(); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	p := url.Values{
		"client_id":    {h.gh.ClientId},
		"redirect_uri": {h.gh.RedirectUri},
		"scope":        {"read:user user:email"},
		"state":        {state},
	}
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+p.Encode(), http.StatusTemporaryRedirect)
}

func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	port, err := h.redis.GetDel(r.Context(), "state_"+state).Result()
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}

	body := url.Values{
		"client_id":     {h.gh.ClientId},
		"client_secret": {h.gh.ClientSecret},
		"code":          {r.URL.Query().Get("code")},
		"redirect_uri":  {h.gh.RedirectUri},
	}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost,
		"https://github.com/login/oauth/access_token", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "token exchange failed")
		return
	}
	defer resp.Body.Close()

	var t struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to decode github response")
		return
	}
	if t.Error != "" {
		utils.WriteError(w, http.StatusBadRequest, t.ErrorDesc)
		return
	}

	ghUser, err := h.fetchGitHubUser(r.Context(), t.AccessToken)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "failed to fetch github user")
		return
	}

	user, err := h.userRepo.GetUserByGithubID(ghUser.ID)
	if err != nil {
		//User doesn't exist. Create them now
		first, err := h.userRepo.IsFirstUser()
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "db error")
			return
		}
		role := data.RoleAnalyst
		if first {
			role = data.RoleAdmin
		}

		user = &data.User{
			Id:       fmt.Sprintf("%d", time.Now().UnixNano()),
			GithubId: ghUser.ID,
			Email:    ghUser.Email,
			Login:    ghUser.Login,
			Role:     role,
		}
		if err := h.userRepo.CreateUser(user); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "failed to create user")
			return
		}
	}

	access, refresh, err := h.issueSession(r.Context(), user)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to issue session")
		return
	}

	if port != "" {
		http.Redirect(w, r,
			fmt.Sprintf("http://localhost:%s/callback?access_token=%s&refresh_token=%s", port, access, refresh),
			http.StatusTemporaryRedirect)
		return
	}

	utils.SetAuthCookies(w, access, refresh)
	utils.WriteSuccess(w, http.StatusOK, "Authentication successful")
}

func (h *AuthHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	userID, err := h.redis.GetDel(r.Context(), "refresh_"+cookie.Value).Result()
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	user, err := h.userRepo.GetUserByID(userID)
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "user not found")
		return
	}

	access, refresh, err := h.issueSession(r.Context(), user)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to issue session")
		return
	}

	utils.SetAuthCookies(w, access, refresh)
	utils.WriteSuccess(w, http.StatusOK, nil)
}

func (h *AuthHandler) HandlePromote(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("id")
	role := data.Role(r.URL.Query().Get("role"))
	if !role.Valid() {
		utils.WriteError(w, http.StatusBadRequest, "invalid role")
		return
	}
	if err := h.userRepo.PromoteUser(userId, role); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to promote user")
		return
	}
	utils.WriteSuccess(w, http.StatusOK, nil)
}

func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := utils.ClaimsFromCtx(r.Context())
	utils.WriteSuccess(w, http.StatusOK, map[string]any{
		"user_id": claims.UserID,
		"role":    claims.Role,
	})
}
func randomBase64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func githubGet(ctx context.Context, client *http.Client, token, endpoint string, dst any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com"+endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (h *AuthHandler) fetchGitHubUser(ctx context.Context, token string) (*githubUser, error) {
	var u githubUser
	if err := githubGet(ctx, h.client, token, "/user", &u); err != nil {
		return nil, err
	}
	if u.Email == "" {
		var emails []struct {
			Email   string `json:"email"`
			Primary bool   `json:"primary"`
		}
		if err := githubGet(ctx, h.client, token, "/user/emails", &emails); err == nil {
			for _, e := range emails {
				if e.Primary {
					u.Email = e.Email
					break
				}
			}
		}
	}
	return &u, nil
}

func (h *AuthHandler) issueSession(ctx context.Context, user *data.User) (access, refresh string, err error) {
	if access, err = utils.NewAccessToken(user.Id, user.Role, []byte(h.jwtSecret)); err != nil {
		return
	}
	if refresh, err = randomBase64(32); err != nil {
		return
	}
	err = h.redis.Set(ctx, "refresh_"+refresh, user.Id, utils.RefreshExpiry).Err()
	return
}
