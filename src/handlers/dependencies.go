package handlers

import (
	"database/sql"
	"net/http"
)

type Dependencies struct {
	Client *http.Client
	DB     *sql.DB
}

func (d *Dependencies) ProfileHandler() ProfileHandler {
	return ProfileHandler{Client: d.Client, db: d.DB}
}

func (d *Dependencies) GenderizeHandler() GenderizeHandler {
	return GenderizeHandler{Client: d.Client}
}
