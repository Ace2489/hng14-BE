package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hng-s1/src/utils"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type ProfileHandler struct {
	Client *http.Client
	db     *sql.DB
}

func ageGroup(age int) string {
	switch {
	case age <= 12:
		return "child"
	case age <= 19:
		return "teenager"
	case age <= 59:
		return "adult"
	default:
		return "senior"
	}
}

func topCountry(countries []country) country {
	best := countries[0]
	for _, c := range countries[1:] {
		if c.Probability > best.Probability {
			best = c
		}
	}
	return best
}

func (h *ProfileHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := utils.LoggerFromCtx(ctx)

	logger.Info("Parsing request body")
	var params ProfileCreateDTO
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		logger.Error("Failed to decode request body", "error", err)
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	logger.Debug(fmt.Sprintf("Request body ->%s<-", params.Name))

	logger.Info("Checking for presence of name in request body")
	if params.Name == "" {
		logger.Error("No name passed in request body")
		utils.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	logger.Info("Validating name in request body")
	for _, c := range params.Name {
		if !unicode.IsLetter(c) {
			logger.Error("Non alphabetical name passed in request body")
			utils.WriteError(w, http.StatusUnprocessableEntity, "name must contain letters only")
			return
		}
	}

	logger.Info("Checking for existing profile for the given name")
	logger.Debug(fmt.Sprintf("Looking up name ->%s<-", params.Name))
	var existing Profile
	err := h.db.QueryRowContext(ctx, `
		SELECT id, name, gender, gender_probability, sample_size, age, age_group,
		       country_id, country_probability, created_at
		FROM profiles WHERE LOWER(name) = LOWER($1)
	`, params.Name).Scan(
		&existing.ID, &existing.Name, &existing.Gender, &existing.GenderProbability,
		&existing.SampleSize, &existing.Age, &existing.AgeGroup,
		&existing.CountryID, &existing.CountryProbability, &existing.CreatedAt,
	)
	if err == nil {
		logger.Info(fmt.Sprintf("Profile already exists for name ->%s<-", params.Name))
		utils.WriteSuccessWithMessage(w, http.StatusOK, "Profile already exists", existing)
		return
	}
	if err != sql.ErrNoRows {
		logger.Error("DB lookup failed", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	logger.Debug(fmt.Sprintf("No existing profile found for name ->%s<-", params.Name))

	var age agifyResponse
	var gender genderizeResponse
	var nationality nationalizeResponse

	type providerConfig struct {
		name     string
		url      string
		target   any
		validate func() bool
	}

	providers := []providerConfig{
		{
			name:     "Agify",
			url:      fmt.Sprintf("https://api.agify.io?name=%s", params.Name),
			target:   &age,
			validate: func() bool { return age.Age != nil && *age.Age != 0 },
		},
		{
			name:     "Genderize",
			url:      fmt.Sprintf("https://api.genderize.io?name=%s", params.Name),
			target:   &gender,
			validate: func() bool { return gender.Gender != nil && gender.Count > 0 },
		},
		{
			name:     "Nationalize",
			url:      fmt.Sprintf("https://api.nationalize.io?name=%s", params.Name),
			target:   &nationality,
			validate: func() bool { return len(nationality.Country) > 0 && nationality.Country[0].CountryID != "" },
		},
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var wg sync.WaitGroup
	ch := make(chan providerResponse, len(providers))

	logger.Info("Concurrently querying external APIs")
	client := h.Client
	for _, cfg := range providers {
		wg.Add(1)
		logger.Debug(fmt.Sprintf("Dispatching request to %s", cfg.name))
		go fetchAndDecode(ctx, client, cfg.name, cfg.url, cfg.target, cfg.validate, &wg, ch)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for res := range ch {
		if res.err != nil {
			cancel()
			logger.Error(fmt.Sprintf("Request to %s failed", res.name), "error", res.err)
			utils.WriteError(w, http.StatusBadGateway, fmt.Sprintf("%s returned an invalid response", res.name))
			return
		}
		logger.Debug(fmt.Sprintf("Request to %s completed successfully", res.name))
	}
	logger.Info("All external API requests completed")

	logger.Info("Building profile from API responses")
	best := topCountry(nationality.Country)
	id, err := uuid.NewV7()
	if err != nil {
		logger.Error("Failed to generate UUID", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "failed to generate id")
		return
	}

	profile := Profile{
		ID:                 id.String(),
		Name:               params.Name,
		Gender:             *gender.Gender,
		GenderProbability:  gender.Probability,
		SampleSize:         gender.Count,
		Age:                *age.Age,
		AgeGroup:           ageGroup(*age.Age),
		CountryID:          best.CountryID,
		CountryProbability: best.Probability,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	logger.Debug(fmt.Sprintf("Profile built ->%+v<-", profile))

	logger.Info("Persisting profile to database")
	_, err = h.db.ExecContext(ctx, `
		INSERT INTO profiles (id, name, gender, gender_probability, sample_size, age, age_group,
		                      country_id, country_probability, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`,
		profile.ID, profile.Name, profile.Gender, profile.GenderProbability,
		profile.SampleSize, profile.Age, profile.AgeGroup,
		profile.CountryID, profile.CountryProbability, profile.CreatedAt,
	)
	if err != nil {
		logger.Error("DB insert failed", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "failed to save profile")
		return
	}
	logger.Info(fmt.Sprintf("Profile persisted successfully with id ->%s<-", profile.ID))

	logger.Info("Writing successful JSON response")
	utils.WriteSuccess(w, http.StatusCreated, profile)
}

func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := utils.LoggerFromCtx(ctx)

	id := r.PathValue("id")
	logger.Info("Fetching profile by ID")
	logger.Debug(fmt.Sprintf("Profile id ->%s<-", id))

	var p Profile
	err := h.db.QueryRowContext(ctx, `
		SELECT id, name, gender, gender_probability, sample_size, age, age_group,
		       country_id, country_probability, created_at
		FROM profiles WHERE id = $1
	`, id).Scan(
		&p.ID, &p.Name, &p.Gender, &p.GenderProbability,
		&p.SampleSize, &p.Age, &p.AgeGroup,
		&p.CountryID, &p.CountryProbability, &p.CreatedAt,
	)
	if err == sql.ErrNoRows {
		logger.Error(fmt.Sprintf("No profile found for id ->%s<-", id))
		utils.WriteError(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		logger.Error("DB query failed", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	logger.Info("Writing successful JSON response")
	utils.WriteSuccess(w, http.StatusOK, p)
}

func (h *ProfileHandler) GetProfiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := utils.LoggerFromCtx(ctx)

	q := r.URL.Query()
	gender := strings.ToLower(q.Get("gender"))
	countryID := strings.ToUpper(q.Get("country_id"))
	ageGrp := strings.ToLower(q.Get("age_group"))

	logger.Info("Parsing query parameters")
	logger.Debug(fmt.Sprintf("Filters -> gender=%s country_id=%s age_group=%s", gender, countryID, ageGrp))

	filters := []string{}
	args := []any{}
	i := 1

	if gender != "" {
		filters = append(filters, fmt.Sprintf("LOWER(gender) = $%d", i))
		args = append(args, gender)
		i++
	}
	if countryID != "" {
		filters = append(filters, fmt.Sprintf("UPPER(country_id) = $%d", i))
		args = append(args, countryID)
		i++
	}
	if ageGrp != "" {
		filters = append(filters, fmt.Sprintf("LOWER(age_group) = $%d", i))
		args = append(args, ageGrp)
		i++
	}

	query := "SELECT id, name, gender, age, age_group, country_id FROM profiles"
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}
	logger.Debug(fmt.Sprintf("Constructed query ->%s<-", query))

	logger.Info("Querying database for profiles")
	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		logger.Error("DB query failed", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	profiles := []ProfileSummary{}
	for rows.Next() {
		var p ProfileSummary
		if err := rows.Scan(&p.ID, &p.Name, &p.Gender, &p.Age, &p.AgeGroup, &p.CountryID); err != nil {
			logger.Error("Failed to scan profile row", "error", err)
			utils.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		profiles = append(profiles, p)
	}
	logger.Debug(fmt.Sprintf("Fetched %d profiles", len(profiles)))

	logger.Info("Writing successful JSON response")
	utils.WriteList(w, http.StatusOK, len(profiles), profiles)
}

func (h *ProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := utils.LoggerFromCtx(ctx)

	id := r.PathValue("id")
	logger.Info("Deleting profile by ID")
	logger.Debug(fmt.Sprintf("Profile id ->%s<-", id))

	res, err := h.db.ExecContext(ctx, "DELETE FROM profiles WHERE id = $1", id)
	if err != nil {
		logger.Error("DB delete failed", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		logger.Error(fmt.Sprintf("No profile found for id ->%s<-", id))
		utils.WriteError(w, http.StatusNotFound, "profile not found")
		return
	}

	logger.Info(fmt.Sprintf("Profile deleted successfully ->%s<-", id))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusNoContent)
}
