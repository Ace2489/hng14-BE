package handlers

import (
	"encoding/json"
	"fmt"
	"hng-s1/src/utils"
	"net/http"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type ProfileHandler struct {
	Client *http.Client
}

type ProfileCreateDTO struct {
	Name string `json:"name"`
}

type Profile struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Gender             string    `json:"gender"`
	GenderProbability  float64   `json:"gender_probability"`
	SampleSize         int       `json:"sample_size"`
	Age                int       `json:"age"`
	AgeGroup           string    `json:"age_group"`
	CountryID          string    `json:"country_id"`
	CountryProbability float64   `json:"country_probability"`
	CreatedAt          string    `json:"created_at"`
}

var cached_name string = ""
var cached_profile Profile = Profile{CreatedAt: time.Now().UTC().Format(time.RFC3339)}

func (p *ProfileHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := utils.LoggerFromCtx(ctx)
	logger.Info("Parsing request body")

	var params ProfileCreateDTO
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		logger.Error("failed to decode request body", "error", err)
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug(fmt.Sprintf("Request body ->%s<-", params))
	if params.Name == "" {
		logger.Error("Invalid name passed in request body")
		utils.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	logger.Info("Validating request body")
	for _, c := range params.Name {
		if !unicode.IsLetter(c) {
			logger.Error("Non alphabetical name passed in request")
			utils.WriteError(w, http.StatusUnprocessableEntity, "name must contain letters only")
			return
		}
	}

	logger.Info("Checking for existing profile for the given input")

	if params.Name == cached_name {
		utils.WriteSuccessWithMessage(w, http.StatusOK, "profile already exists", cached_profile)
		return
	}

	logger.Info("Creating a new profile for the given input")
	logger.Debug(fmt.Sprintf("Cached name {%s}", cached_name))
	cached_name, cached_profile.Name = params.Name, params.Name
	utils.WriteSuccess(w, http.StatusCreated, cached_profile)

}
