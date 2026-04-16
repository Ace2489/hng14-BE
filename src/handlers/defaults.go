package handlers

import (
	"hng-s1/src/utils"
	"net/http"
)

// Default Handlers
func HandleNotFound(w http.ResponseWriter, r *http.Request) {
	utils.WriteError(w, http.StatusNotFound, "route not found")
}
