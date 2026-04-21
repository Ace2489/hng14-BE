package handlers

type genderData struct {
	Name        string  `json:"name"`
	Gender      string  `json:"gender"`
	Probability float64 `json:"probability"`
	SampleSize  int     `json:"sample_size"`
	IsConfident bool    `json:"is_confident"`
	ProcessedAt string  `json:"processed_at"`
}

type ProfileCreateDTO struct {
	Name string `json:"name"`
}

type country struct {
	CountryID   string  `json:"country_id"`
	Probability float64 `json:"probability"`
}

type nationalizeResponse struct {
	Count   int       `json:"count"`
	Name    string    `json:"name"`
	Country []country `json:"country"`
}

type agifyResponse struct {
	Count *int   `json:"count"`
	Name  string `json:"name"`
	Age   *int   `json:"age"`
}

type genderizeResponse struct {
	Count       int     `json:"count"`
	Name        string  `json:"name"`
	Gender      *string `json:"gender"`
	Probability float64 `json:"probability"`
}

type Profile struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Gender             string  `json:"gender"`
	GenderProbability  float64 `json:"gender_probability"`
	Age                int     `json:"age"`
	AgeGroup           string  `json:"age_group"`
	CountryID          string  `json:"country_id"`
	CountryName        string  `json:"country_name"`
	CountryProbability float64 `json:"country_probability"`
	CreatedAt          string  `json:"created_at"`
}

type ProfileSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Gender    string `json:"gender"`
	Age       int    `json:"age"`
	AgeGroup  string `json:"age_group"`
	CountryID string `json:"country_id"`
}

type providerResponse struct {
	name string
	err  error
}
