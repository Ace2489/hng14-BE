package handlers

import (
	"encoding/json"
	"fmt"
	"hng-s1/src/utils"
	"io"
	"net/http"
	"time"
	"unicode"
)

type GenderizeHandler struct {
	Client *http.Client
}

func (s *GenderizeHandler) HandleGender(w http.ResponseWriter, r *http.Request) {
	logger := utils.LoggerFromCtx(r.Context())
	name := r.URL.Query().Get("name")

	logger.Info("Checking for presence of name in query params")
	debugMsg := fmt.Sprintf("Name {%s}", name)
	logger.Debug(debugMsg)
	if name == "" {
		logger.Error("No name passed in request")
		utils.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	logger.Info("Validating name in query params")
	for _, c := range name {
		if !unicode.IsLetter(c) {
			logger.Error("Non alphabetical name passed in request")
			utils.WriteError(w, http.StatusUnprocessableEntity, "name must contain letters only")
			return
		}
	}

	logger.Info(fmt.Sprintf("Querying Genderize API for name=%s", name))
	resp, err := s.Client.Get("https://api.genderize.io/?name=" + name)
	if err != nil {
		msg := "failed to reach gender prediction service"
		logger.Error(msg)
		utils.WriteError(w, http.StatusBadGateway, msg)
		return
	}
	defer resp.Body.Close()

	logger.Debug("Reading response from Genderize")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msg := "failed to read prediction response"
		logger.Error(msg)
		utils.WriteError(w, http.StatusInternalServerError, msg)
		return
	}

	var gr genderizeResponse
	logger.Info("Parsing and processing response from Genderize")
	if err := json.Unmarshal(body, &gr); err != nil {
		msg := "failed to parse prediction response"
		utils.WriteError(w, http.StatusInternalServerError, msg)
		return
	}

	if gr.Gender == nil || gr.Count == 0 {
		msg := "No prediction available for the provided name"
		logger.Error(msg)
		utils.WriteError(w, http.StatusInternalServerError, msg)
		return
	}

	logger.Info("Writing successful JSON response")
	utils.WriteSuccess(w, http.StatusOK, genderData{
		Name:        gr.Name,
		Gender:      *gr.Gender,
		Probability: gr.Probability,
		SampleSize:  gr.Count,
		IsConfident: gr.Probability >= 0.7 && gr.Count >= 100,
		ProcessedAt: time.Now().UTC().Format(time.RFC3339),
	})
}
