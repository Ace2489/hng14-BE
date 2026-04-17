package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	port = flag.String("port", "3000", "API server port")
	host = flag.String("host", "http://localhost", "API host")
)

func classifyURL() string { return fmt.Sprintf("%s:%s/api/classify", *host, *port) }
func profilesURL() string { return fmt.Sprintf("%s:%s/api/profiles", *host, *port) }

/*
------------------------------------------------------------
SHARED TEST STATE (Stage 1)
------------------------------------------------------------
*/

var (
	createdID      string
	createdProfile Profile
)

/*
------------------------------------------------------------
MODELS
------------------------------------------------------------
*/

// Stage 0
type SuccessResponse struct {
	Status string `json:"status"`
	Data   struct {
		Name        string  `json:"name"`
		Gender      string  `json:"gender"`
		Probability float64 `json:"probability"`
		SampleSize  int     `json:"sample_size"`
		IsConfident bool    `json:"is_confident"`
		ProcessedAt string  `json:"processed_at"`
	} `json:"data"`
}

// Stage 1
type Profile struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Gender             string  `json:"gender"`
	GenderProbability  float64 `json:"gender_probability"`
	SampleSize         int     `json:"sample_size"`
	Age                int     `json:"age"`
	AgeGroup           string  `json:"age_group"`
	CountryID          string  `json:"country_id"`
	CountryProbability float64 `json:"country_probability"`
	CreatedAt          string  `json:"created_at"`
}

type ProfileResp struct {
	Status  string  `json:"status"`
	Message string  `json:"message"`
	Data    Profile `json:"data"`
}

type ProfilesResp struct {
	Status string    `json:"status"`
	Count  int       `json:"count"`
	Data   []Profile `json:"data"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

/*
------------------------------------------------------------
HTTP CLIENT
------------------------------------------------------------
*/

func get(url string) (*http.Response, []byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp, b, err
}

func post(url, body string) (*http.Response, []byte, error) {
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp, b, err
}

func del(url string) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	return resp, err
}

func decode[T any](data []byte) (T, error) {
	var v T
	return v, json.Unmarshal(data, &v)
}

/*
------------------------------------------------------------
ASSERTIONS
------------------------------------------------------------
*/

func fail(test, reason, expected, got string, body []byte) {
	s := fmt.Sprintf("\n[FAIL] %s\n  reason:   %s\n  expected: %s\n  got:      %s\n", test, reason, expected, got)
	if len(body) > 0 {
		s += fmt.Sprintf("  body:     %s\n", body)
	}
	fmt.Print(s)
	os.Exit(1)
}

func pass(name string) { fmt.Printf("[PASS] %s\n", name) }

func assertStatus(test string, got, want int, body []byte) {
	if got != want {
		fail(test, "status code", fmt.Sprint(want), fmt.Sprint(got), body)
	}
}

func assertNonEmpty(test, field, val string, body []byte) {
	if val == "" {
		fail(test, field+" is empty", "non-empty", `""`, body)
	}
}

func isUUIDv7(s string) bool {
	return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-' && s[14] == '7'
}

func validAgeGroup(age int, group string) bool {
	switch {
	case age <= 12:
		return group == "child"
	case age <= 19:
		return group == "teenager"
	case age <= 59:
		return group == "adult"
	default:
		return group == "senior"
	}
}

/*
------------------------------------------------------------
STAGE 0 TESTS (/api/classify)
------------------------------------------------------------
*/

func testValidRequest() {
	resp, body, err := get(classifyURL() + "?name=John")
	if err != nil {
		fail("VALID_REQUEST", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("VALID_REQUEST", resp.StatusCode, 200, body)

	r, err := decode[SuccessResponse](body)
	if err != nil {
		fail("VALID_REQUEST", "invalid JSON", "SuccessResponse", err.Error(), body)
	}
	if r.Status != "success" {
		fail("VALID_REQUEST", "status field", "success", r.Status, body)
	}
	assertNonEmpty("VALID_REQUEST", "data.name", r.Data.Name, body)
	if r.Data.Gender != "male" && r.Data.Gender != "female" {
		fail("VALID_REQUEST", "data.gender", "male|female", r.Data.Gender, body)
	}
	if r.Data.Probability < 0 || r.Data.Probability > 1 {
		fail("VALID_REQUEST", "data.probability range", "0.0–1.0", fmt.Sprintf("%.4f", r.Data.Probability), body)
	}
	if r.Data.SampleSize <= 0 {
		fail("VALID_REQUEST", "data.sample_size", "> 0", fmt.Sprint(r.Data.SampleSize), body)
	}
	assertNonEmpty("VALID_REQUEST", "data.processed_at", r.Data.ProcessedAt, body)
	pass("VALID_REQUEST")
}

func testIsConfidentLogic() {
	resp, body, err := get(classifyURL() + "?name=John")
	if err != nil {
		fail("CONFIDENCE_LOGIC", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("CONFIDENCE_LOGIC", resp.StatusCode, 200, body)

	r, _ := decode[SuccessResponse](body)
	p, s := r.Data.Probability, r.Data.SampleSize
	detail := fmt.Sprintf("probability=%.2f sample_size=%d", p, s)

	if r.Data.IsConfident {
		if p < 0.7 || s < 100 {
			fail("CONFIDENCE_LOGIC", "is_confident=true but thresholds not met", "p>=0.7 AND n>=100", detail, body)
		}
	} else {
		if p >= 0.7 && s >= 100 {
			fail("CONFIDENCE_LOGIC", "is_confident=false but both thresholds met", "p<0.7 OR n<100", detail, body)
		}
	}
	pass("CONFIDENCE_LOGIC")
}

func testMissingName() {
	resp, body, err := get(classifyURL())
	if err != nil {
		fail("MISSING_NAME", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("MISSING_NAME", resp.StatusCode, 400, body)
	pass("MISSING_NAME")
}

func testInvalidName() {
	resp, body, err := get(classifyURL() + "?name=1234")
	if err != nil {
		fail("INVALID_NAME", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("INVALID_NAME", resp.StatusCode, 422, body)
	pass("INVALID_NAME")
}

func testEdgeCase() {
	resp, body, err := get(classifyURL() + "?name=asdkfjhasdkjfh")
	if err != nil {
		fail("EDGE_CASE", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("EDGE_CASE", resp.StatusCode, 500, body)

	r, err := decode[ErrorResponse](body)
	if err != nil {
		fail("EDGE_CASE", "invalid error JSON", "ErrorResponse", err.Error(), body)
	}
	if r.Status != "error" {
		fail("EDGE_CASE", "status field", "error", r.Status, body)
	}
	const wantMsg = "No prediction available for the provided name"
	if !strings.EqualFold(r.Message, wantMsg) {
		fail("EDGE_CASE", "error message", wantMsg, r.Message, body)
	}
	pass("EDGE_CASE")
}

func testTimestampFormat() {
	resp, body, err := get(classifyURL() + "?name=John")
	if err != nil {
		fail("TIMESTAMP", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("TIMESTAMP", resp.StatusCode, 200, body)

	r, _ := decode[SuccessResponse](body)
	if _, err := time.Parse(time.RFC3339, r.Data.ProcessedAt); err != nil {
		fail("TIMESTAMP", "invalid timestamp", "RFC3339", r.Data.ProcessedAt, body)
	}
	pass("TIMESTAMP")
}

/*
------------------------------------------------------------
STAGE 1 TESTS (/api/profiles)
------------------------------------------------------------
*/

const testName = "ella"

func testCreateProfile() {
	resp, body, err := post(profilesURL(), `{"name":"`+testName+`"}`)
	if err != nil {
		fail("CREATE_PROFILE", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("CREATE_PROFILE", resp.StatusCode, 201, body)

	r, err := decode[ProfileResp](body)
	if err != nil {
		fail("CREATE_PROFILE", "invalid JSON", "ProfileResp", err.Error(), body)
	}
	if r.Status != "success" {
		fail("CREATE_PROFILE", "status field", "success", r.Status, body)
	}

	p := r.Data
	if !isUUIDv7(p.ID) {
		fail("CREATE_PROFILE", "id not UUID v7", "xxxxxxxx-xxxx-7xxx-...", p.ID, body)
	}
	assertNonEmpty("CREATE_PROFILE", "name", p.Name, body)
	if p.Gender != "male" && p.Gender != "female" {
		fail("CREATE_PROFILE", "gender", "male|female", p.Gender, body)
	}
	if p.GenderProbability < 0 || p.GenderProbability > 1 {
		fail("CREATE_PROFILE", "gender_probability range", "0.0–1.0", fmt.Sprintf("%.4f", p.GenderProbability), body)
	}
	if p.SampleSize <= 0 {
		fail("CREATE_PROFILE", "sample_size", "> 0", fmt.Sprint(p.SampleSize), body)
	}
	if p.Age <= 0 {
		fail("CREATE_PROFILE", "age", "> 0", fmt.Sprint(p.Age), body)
	}
	if !validAgeGroup(p.Age, p.AgeGroup) {
		fail("CREATE_PROFILE", "age_group inconsistent with age",
			fmt.Sprintf("correct group for age %d", p.Age), p.AgeGroup, body)
	}
	assertNonEmpty("CREATE_PROFILE", "country_id", p.CountryID, body)
	if p.CountryProbability < 0 || p.CountryProbability > 1 {
		fail("CREATE_PROFILE", "country_probability range", "0.0–1.0", fmt.Sprintf("%.4f", p.CountryProbability), body)
	}
	if _, err := time.Parse(time.RFC3339, p.CreatedAt); err != nil {
		fail("CREATE_PROFILE", "created_at format", "RFC3339", p.CreatedAt, body)
	}

	createdID = p.ID
	createdProfile = p
	pass("CREATE_PROFILE")
}

func testIdempotency() {
	resp, body, err := post(profilesURL(), `{"name":"`+testName+`"}`)
	if err != nil {
		fail("IDEMPOTENCY", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("IDEMPOTENCY", resp.StatusCode, 200, body)

	r, err := decode[ProfileResp](body)
	if err != nil {
		fail("IDEMPOTENCY", "invalid JSON", "ProfileResp", err.Error(), body)
	}
	if r.Status != "success" {
		fail("IDEMPOTENCY", "status field", "success", r.Status, body)
	}
	if r.Message == "" {
		fail("IDEMPOTENCY", "missing message on duplicate", "non-empty message", `""`, body)
	}
	if r.Data.ID != createdID {
		fail("IDEMPOTENCY", "id changed on duplicate", createdID, r.Data.ID, body)
	}
	pass("IDEMPOTENCY")
}

func testGetProfile() {
	resp, body, err := get(profilesURL() + "/" + createdID)
	if err != nil {
		fail("GET_PROFILE", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("GET_PROFILE", resp.StatusCode, 200, body)

	r, err := decode[ProfileResp](body)
	if err != nil {
		fail("GET_PROFILE", "invalid JSON", "ProfileResp", err.Error(), body)
	}
	if r.Data.ID != createdID {
		fail("GET_PROFILE", "id mismatch", createdID, r.Data.ID, body)
	}
	if r.Data.Name != createdProfile.Name {
		fail("GET_PROFILE", "name mismatch", createdProfile.Name, r.Data.Name, body)
	}
	pass("GET_PROFILE")
}

func testGetAllProfiles() {
	resp, body, err := get(profilesURL())
	if err != nil {
		fail("GET_ALL_PROFILES", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("GET_ALL_PROFILES", resp.StatusCode, 200, body)

	r, err := decode[ProfilesResp](body)
	if err != nil {
		fail("GET_ALL_PROFILES", "invalid JSON", "ProfilesResp", err.Error(), body)
	}
	if r.Status != "success" {
		fail("GET_ALL_PROFILES", "status field", "success", r.Status, body)
	}
	if r.Count != len(r.Data) {
		fail("GET_ALL_PROFILES", "count != len(data)", fmt.Sprint(len(r.Data)), fmt.Sprint(r.Count), body)
	}
	if r.Count < 1 {
		fail("GET_ALL_PROFILES", "count too low", ">= 1", fmt.Sprint(r.Count), body)
	}
	pass("GET_ALL_PROFILES")
}

func testFilterProfiles() {
	gender := createdProfile.Gender
	ageGroup := createdProfile.AgeGroup
	countryID := createdProfile.CountryID

	// Case-insensitive gender filter — uppercase first letter
	mixedGender := strings.ToUpper(gender[:1]) + gender[1:]
	url := fmt.Sprintf("%s?gender=%s", profilesURL(), mixedGender)
	resp, body, err := get(url)
	if err != nil {
		fail("FILTER_GENDER_CASE", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("FILTER_GENDER_CASE", resp.StatusCode, 200, body)
	r, err := decode[ProfilesResp](body)
	if err != nil {
		fail("FILTER_GENDER_CASE", "invalid JSON", "ProfilesResp", err.Error(), body)
	}
	found := false
	for _, p := range r.Data {
		if p.ID == createdID {
			found = true
		}
		if !strings.EqualFold(p.Gender, gender) {
			fail("FILTER_GENDER_CASE", "non-matching gender in results", gender, p.Gender, body)
		}
	}
	if !found {
		fail("FILTER_GENDER_CASE", "created profile missing from filtered results", createdID, "not found", body)
	}
	pass("FILTER_GENDER_CASE")

	// country_id filter
	resp, body, err = get(fmt.Sprintf("%s?country_id=%s", profilesURL(), countryID))
	if err != nil {
		fail("FILTER_COUNTRY", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("FILTER_COUNTRY", resp.StatusCode, 200, body)
	r, _ = decode[ProfilesResp](body)
	for _, p := range r.Data {
		if !strings.EqualFold(p.CountryID, countryID) {
			fail("FILTER_COUNTRY", "non-matching country_id in results", countryID, p.CountryID, body)
		}
	}
	pass("FILTER_COUNTRY")

	// age_group filter
	resp, body, err = get(fmt.Sprintf("%s?age_group=%s", profilesURL(), ageGroup))
	if err != nil {
		fail("FILTER_AGE_GROUP", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("FILTER_AGE_GROUP", resp.StatusCode, 200, body)
	r, _ = decode[ProfilesResp](body)
	for _, p := range r.Data {
		if !strings.EqualFold(p.AgeGroup, ageGroup) {
			fail("FILTER_AGE_GROUP", "non-matching age_group in results", ageGroup, p.AgeGroup, body)
		}
	}
	pass("FILTER_AGE_GROUP")

	// combined filter
	resp, body, err = get(fmt.Sprintf("%s?gender=%s&country_id=%s", profilesURL(), gender, countryID))
	if err != nil {
		fail("FILTER_COMBINED", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("FILTER_COMBINED", resp.StatusCode, 200, body)
	r, _ = decode[ProfilesResp](body)
	for _, p := range r.Data {
		if !strings.EqualFold(p.Gender, gender) || !strings.EqualFold(p.CountryID, countryID) {
			fail("FILTER_COMBINED", "result violates combined filter", gender+"+"+countryID,
				p.Gender+"+"+p.CountryID, body)
		}
	}
	pass("FILTER_COMBINED")
}

func testProfileNotFound() {
	resp, body, err := get(profilesURL() + "/00000000-0000-7000-0000-000000000000")
	if err != nil {
		fail("PROFILE_NOT_FOUND", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("PROFILE_NOT_FOUND", resp.StatusCode, 404, body)
	r, err := decode[ErrorResponse](body)
	if err != nil {
		fail("PROFILE_NOT_FOUND", "invalid error JSON", "ErrorResponse", err.Error(), body)
	}
	if r.Status != "error" {
		fail("PROFILE_NOT_FOUND", "status field", "error", r.Status, body)
	}
	pass("PROFILE_NOT_FOUND")
}

func testProfileMissingName() {
	resp, body, err := post(profilesURL(), `{}`)
	if err != nil {
		fail("PROFILE_MISSING_NAME", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("PROFILE_MISSING_NAME", resp.StatusCode, 400, body)
	pass("PROFILE_MISSING_NAME")
}

func testProfileEmptyName() {
	resp, body, err := post(profilesURL(), `{"name":""}`)
	if err != nil {
		fail("PROFILE_EMPTY_NAME", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("PROFILE_EMPTY_NAME", resp.StatusCode, 400, body)
	pass("PROFILE_EMPTY_NAME")
}

func testProfileInvalidName() {
	resp, body, err := post(profilesURL(), `{"name":"1234"}`)
	if err != nil {
		fail("PROFILE_INVALID_NAME", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("PROFILE_INVALID_NAME", resp.StatusCode, 422, body)
	pass("PROFILE_INVALID_NAME")
}

func testCORSHeader() {
	resp, _, err := get(profilesURL())
	if err != nil {
		fail("CORS_HEADER", "HTTP failure", "no error", err.Error(), nil)
	}
	if v := resp.Header.Get("Access-Control-Allow-Origin"); v != "*" {
		fail("CORS_HEADER", "missing or wrong CORS header", "*", v, nil)
	}
	pass("CORS_HEADER")
}

func testDeleteProfile() {
	resp, err := del(profilesURL() + "/" + createdID)
	if err != nil {
		fail("DELETE_PROFILE", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("DELETE_PROFILE", resp.StatusCode, 204, nil)
	pass("DELETE_PROFILE")
}

func testDeletedProfileIsGone() {
	resp, body, err := get(profilesURL() + "/" + createdID)
	if err != nil {
		fail("DELETED_PROFILE_GONE", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("DELETED_PROFILE_GONE", resp.StatusCode, 404, body)
	pass("DELETED_PROFILE_GONE")
}

func testDeleteNotFound() {
	resp, err := del(profilesURL() + "/00000000-0000-7000-0000-000000000000")
	if err != nil {
		fail("DELETE_NOT_FOUND", "HTTP failure", "no error", err.Error(), nil)
	}
	assertStatus("DELETE_NOT_FOUND", resp.StatusCode, 404, nil)
	pass("DELETE_NOT_FOUND")
}

/*
------------------------------------------------------------
ENTRYPOINT
------------------------------------------------------------
*/

func main() {
	flag.Parse()
	fmt.Println("Running tests against:", *host+":"+*port)

	fmt.Println("\n--- Stage 0: /api/classify ---")
	testValidRequest()
	testIsConfidentLogic()
	testMissingName()
	testInvalidName()
	testEdgeCase()
	testTimestampFormat()

	fmt.Println("\n--- Stage 1: /api/profiles ---")
	testCreateProfile()
	testIdempotency()
	testGetProfile()
	testGetAllProfiles()
	testFilterProfiles()
	testProfileNotFound()
	testProfileMissingName()
	testProfileEmptyName()
	testProfileInvalidName()
	testCORSHeader()
	testDeleteProfile()
	testDeletedProfileIsGone()
	testDeleteNotFound()

	fmt.Println("\nALL TESTS PASSED")
}
