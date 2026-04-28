package handlers

import (
	"database/sql"
	"hng-s1/src/utils"
	"net/http"

	"github.com/redis/go-redis/v9"
)

type Dependencies struct {
	Client *http.Client
	DB     *sql.DB
	Redis  *redis.Client
	Gh     *utils.GithubOauth
}

func (d *Dependencies) ProfileHandler() ProfileHandler {
	return ProfileHandler{Client: d.Client, db: d.DB}
}

func (d *Dependencies) GenderizeHandler() GenderizeHandler {
	return GenderizeHandler{Client: d.Client}
}

func (d *Dependencies) AuthHandler() AuthHandler {
	return AuthHandler{redis: d.Redis, gh: d.Gh, client: d.Client}
}
