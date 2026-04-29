package handlers

import (
	"database/sql"
	"hng-s1/src/data"
	"hng-s1/src/utils"
	"net/http"

	"github.com/redis/go-redis/v9"
)

type Dependencies struct {
	Client    *http.Client
	DB        *sql.DB
	Redis     *redis.Client
	Gh        *utils.GithubOauth
	JwtSecret string
}

func (d *Dependencies) ProfileHandler() ProfileHandler {
	return ProfileHandler{Client: d.Client, db: d.DB}
}

func (d *Dependencies) GenderizeHandler() GenderizeHandler {
	return GenderizeHandler{Client: d.Client}
}

func (d *Dependencies) AuthHandler() AuthHandler {
	return AuthHandler{redis: d.Redis, db: d.DB, gh: d.Gh, jwtSecret: d.JwtSecret, client: d.Client, userRepo: &data.UserRepo{DB: d.DB}}
}
