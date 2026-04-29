package handlers

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"hng-s1/src/utils"
	"net/http"
	"net/url"
	"strconv"
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
		SELECT id, name, gender, gender_probability, age, age_group,
		       country_id, country_name, country_probability, created_at
		FROM profiles WHERE LOWER(name) = LOWER($1)
	`, params.Name).Scan(
		&existing.ID, &existing.Name, &existing.Gender, &existing.GenderProbability,
		&existing.Age, &existing.AgeGroup,
		&existing.CountryID, &existing.CountryName, &existing.CountryProbability, &existing.CreatedAt,
	)
	if err == nil {
		logger.Info(fmt.Sprintf("Profile already exists for name ->%s<-", params.Name))
		utils.WriteSuccessWithMessage(w, http.StatusOK, "Profile already exists", existing)
		return
	}
	if err != sql.ErrNoRows {
		logger.Error("DB lookup failed", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "Internal server error")
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
	country := nationality.Country[0]
	country_name, ok := utils.Alpha2ToName(country.CountryID)
	logger.Debug(fmt.Sprintf("Country name: %s", country_name))
	if !ok {
		logger.Error("Failed to get country name from country code")
		utils.WriteError(w, http.StatusInternalServerError, "Something went wrong")
	}

	id, err := uuid.NewV7()
	if err != nil {
		logger.Error("Failed to generate UUID", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "failed to generate id") //This is more information than the client should be receiving, no?
		return
	}

	profile := Profile{
		ID:                 id.String(),
		Name:               params.Name,
		Gender:             *gender.Gender,
		GenderProbability:  gender.Probability,
		Age:                *age.Age,
		AgeGroup:           ageGroup(*age.Age),
		CountryID:          country.CountryID,
		CountryName:        country_name,
		CountryProbability: country.Probability,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	logger.Debug(fmt.Sprintf("Profile built ->%+v<-", profile))

	logger.Info("Persisting profile to database")
	_, err = h.db.ExecContext(ctx, `
		INSERT INTO profiles (id, name, gender, gender_probability, age, age_group,
		                      country_id, country_name, country_probability, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`,
		profile.ID, profile.Name, profile.Gender, profile.GenderProbability,
		profile.Age, profile.AgeGroup,
		profile.CountryID, profile.CountryName, profile.CountryProbability, profile.CreatedAt,
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
		SELECT id, name, gender, gender_probability, age, age_group,
		       country_id, country_name, country_probability, created_at
		FROM profiles WHERE id = $1
	`, id).Scan(
		&p.ID, &p.Name, &p.Gender, &p.GenderProbability,
		&p.Age, &p.AgeGroup,
		&p.CountryID, &p.CountryName, &p.CountryProbability, &p.CreatedAt,
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
	h.fetchProfiles(r.Context(), r.URL.Query(), w)
}

func (h *ProfileHandler) SearchProfiles(w http.ResponseWriter, r *http.Request) {
	logger := utils.LoggerFromCtx(r.Context())
	raw := r.URL.Query().Get("q")

	logger.Info("Natural language search", "q", raw)

	filters, ok := parseLanguageQuery(raw)
	if !ok {
		logger.Error("Unable to interpret query", "q", raw)
		utils.WriteError(w, http.StatusBadRequest, "Unable to interpret query")
		return
	}

	logger.DebugFmt("Parsed NL filters -> %s", filters.Encode())

	for _, k := range []string{"page", "limit", "sort_by", "order"} {
		if v := r.URL.Query().Get(k); v != "" {
			filters.Set(k, v)
		}
	}
	h.fetchProfiles(r.Context(), filters, w)
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
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfileHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	logger := utils.LoggerFromCtx(r.Context())
	id := r.URL.Query().Get("id")
	profiles, err := h.GetProfilesForExport(id, logger)

	if err != nil {
		logger.Error("Error exporting profiles", "error:", err)
		utils.WriteError(w, http.StatusInternalServerError, "export failed")
		return
	}
	if len(profiles) == 0 {
		logger.Error("No profiles matched the given id", "id:", id)
		utils.WriteError(w, http.StatusInternalServerError, "No profiles matched the given id")
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=profiles.csv")

	cw := csv.NewWriter(w)
	cw.Write([]string{"id", "name", "gender", "gender_probability", "age", "age_group", "country_id", "country_name", "country_probability", "created_at"})
	for _, p := range profiles {
		cw.Write([]string{
			p.ID, p.Name, p.Gender,
			strconv.FormatFloat(p.GenderProbability, 'f', 4, 64),
			strconv.Itoa(p.Age), p.AgeGroup,
			p.CountryID, p.CountryName,
			strconv.FormatFloat(p.CountryProbability, 'f', 4, 64),
			p.CreatedAt,
		})
	}
	logger.Info("Sending profiles.csv to the client")
	cw.Flush()
}
func (h *ProfileHandler) fetchProfiles(ctx context.Context, q url.Values, w http.ResponseWriter) {
	logger := utils.LoggerFromCtx(ctx)

	page := queryInt(q, "page", 1)
	limit := min(queryInt(q, "limit", 10), 50)

	sortCol := map[string]string{
		"age": "age", "created_at": "created_at", "gender_probability": "gender_probability",
	}[q.Get("sort_by")]
	if sortCol == "" {
		sortCol = "created_at"
	}
	order := "ASC"
	if strings.ToLower(q.Get("order")) == "desc" {
		order = "DESC"
	}

	var where []string
	var args []any
	invalid := false

	strFilter := func(col, val string) {
		if val != "" {
			where = append(where, col+" = ?")
			args = append(args, val)
		}
	}
	rangeFilter := func(col, op, val string, parse func(string) (any, error)) {
		if val == "" {
			return
		}
		v, err := parse(val)
		if err != nil {
			invalid = true
			return
		}
		where = append(where, col+" "+op+" ?")
		args = append(args, v)
	}

	strFilter("gender", strings.ToLower(q.Get("gender")))
	strFilter("age_group", strings.ToLower(q.Get("age_group")))
	strFilter("country_id", strings.ToUpper(q.Get("country_id")))
	rangeFilter("age", ">=", q.Get("min_age"), func(s string) (any, error) { n, err := strconv.Atoi(s); return n, err })
	rangeFilter("age", "<=", q.Get("max_age"), func(s string) (any, error) { n, err := strconv.Atoi(s); return n, err })
	rangeFilter("gender_probability", ">=", q.Get("min_gender_probability"), func(s string) (any, error) { return strconv.ParseFloat(s, 64) })
	rangeFilter("country_probability", ">=", q.Get("min_country_probability"), func(s string) (any, error) { return strconv.ParseFloat(s, 64) })

	if invalid {
		logger.Error("Invalid query parameters", "params", q.Encode())
		utils.WriteError(w, http.StatusBadRequest, "Invalid query parameters")
		return
	}

	clause := "FROM profiles"
	if len(where) > 0 {
		clause += " WHERE " + strings.Join(where, " AND ")
	}

	logger.Info("Querying profiles", "page", page, "limit", limit, "sort_by", sortCol, "order", order)
	logger.DebugFmt("Filters -> %s | clause -> %s", q.Encode(), clause)

	logger.DebugFmt("	")
	rows, err := h.db.QueryContext(ctx, fmt.Sprintf(
		"SELECT id, name, gender, gender_probability, age, age_group, country_id, country_name, country_probability, created_at, COUNT(*) OVER() %s ORDER BY %s %s LIMIT ? OFFSET ?",
		clause, sortCol, order,
	), append(args, limit, (page-1)*limit)...)
	if err != nil {
		logger.Error("DB query failed", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	var total int
	profiles := []Profile{}
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.ID, &p.Name, &p.Gender, &p.GenderProbability, &p.Age, &p.AgeGroup, &p.CountryID, &p.CountryName, &p.CountryProbability, &p.CreatedAt, &total); err != nil {
			logger.Error("Row scan failed", "error", err)
			utils.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		profiles = append(profiles, p)
	}

	logger.Info("Profiles fetched", "count", len(profiles), "total", total)
	utils.WritePaginatedResponse(w, http.StatusOK, page, limit, total, profiles)
}

func (h *ProfileHandler) GetProfilesForExport(id string, logger utils.Logger) ([]Profile, error) {
	query := "SELECT id, name, gender, gender_probability, age, age_group, country_id, country_name, country_probability, created_at FROM profiles"
	args := []any{}
	if id != "" {
		query += " WHERE id = ?"
		args = append(args, id)
	}
	logger.InfoFmt("Profile export query %s", query)
	rows, err := h.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var profiles []Profile
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.ID, &p.Name, &p.Gender, &p.GenderProbability, &p.Age, &p.AgeGroup, &p.CountryID, &p.CountryName, &p.CountryProbability, &p.CreatedAt); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	logger.InfoFmt("Returning %d profiles for export", len(profiles))
	return profiles, rows.Err()
}
func queryInt(q url.Values, key string, def int) int {
	v, err := strconv.Atoi(q.Get(key))
	if err != nil || v < 1 {
		return def
	}
	return v
}

// Temp: Get the version from Flaticols later
var countryMap = map[string]string{
	"nigeria": "NG", "tanzania": "TZ", "kenya": "KE", "angola": "AO",
	"benin": "BJ", "ghana": "GH", "ethiopia": "ET", "uganda": "UG",
	"senegal": "SN", "cameroon": "CM", "ivory coast": "CI", "mali": "ML",
	"mozambique": "MZ", "zambia": "ZM", "zimbabwe": "ZW", "rwanda": "RW",
	"somalia": "SO", "sudan": "SD", "tunisia": "TN", "algeria": "DZ",
	"morocco": "MA", "egypt": "EG", "south africa": "ZA", "niger": "NE",
	"chad": "TD", "burkina faso": "BF", "malawi": "MW", "togo": "TG",
}

func parseLanguageQuery(q string) (url.Values, bool) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil, false
	}

	filters := url.Values{}
	words := strings.Fields(q)

	if strings.Contains(q, "female") {
		filters.Set("gender", "female")
	} else if strings.Contains(q, "male") {
		filters.Set("gender", "male")
	}

	for _, ag := range []string{"child", "teenager", "adult", "senior"} {
		if strings.Contains(q, ag) {
			filters.Set("age_group", ag)
			break
		}
	}

	if strings.Contains(q, "young") {
		filters.Set("min_age", "16")
		filters.Set("max_age", "24")
	}
	for i, w := range words {
		if i+1 >= len(words) {
			break
		}
		n, err := strconv.Atoi(words[i+1])
		if err != nil {
			continue
		}
		switch w {
		case "above", "over":
			filters.Set("min_age", strconv.Itoa(n))
		case "below", "under":
			filters.Set("max_age", strconv.Itoa(n))
		}
	}

	if idx := strings.Index(q, "from "); idx != -1 {
		parts := strings.Fields(q[idx+5:])
		for l := len(parts); l > 0; l-- {
			if code, ok := countryMap[strings.Join(parts[:l], " ")]; ok {
				filters.Set("country_id", code)
				break
			}
		}
	}

	if len(filters) == 0 {
		return nil, false
	}

	return filters, true
}
