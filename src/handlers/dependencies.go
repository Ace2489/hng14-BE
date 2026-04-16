package handlers

import "net/http"

type Dependencies struct {
	Client *http.Client
}

func (d *Dependencies) ProfileHandler() ProfileHandler {
	return ProfileHandler{Client: d.Client}
}

func (d *Dependencies) GenderizeHandler() GenderizeHandler {
	return GenderizeHandler{Client: d.Client}
}
