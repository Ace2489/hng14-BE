package main

import (
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

func baseURL() string {
	return fmt.Sprintf("%s:%s/api/classify", *host, *port)
}

type TestError struct {
	Test     string
	Reason   string
	Expected string
	Got      string
	Body     []byte
}

func (e TestError) Error() string {
	s := fmt.Sprintf(
		"\n[TEST FAILED]\nTest:     %s\nReason:   %s\nExpected: %s\nGot:      %s\n",
		e.Test, e.Reason, e.Expected, e.Got,
	)
	if len(e.Body) > 0 {
		s += fmt.Sprintf("Response: %s\n", e.Body)
	}
	return s
}

func fail(test, reason, expected, got string, body []byte) {
	fmt.Println(TestError{test, reason, expected, got, body}.Error())
	os.Exit(1)
}

func pass(name string) {
	fmt.Printf("[PASS] %s\n", name)
}

/*
------------------------------------------------------------
API MODELS
------------------------------------------------------------
*/

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return resp, body, nil
}

func decodeSuccess(data []byte) (SuccessResponse, error) {
	var r SuccessResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func decodeError(data []byte) (ErrorResponse, error) {
	var r ErrorResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

/*
------------------------------------------------------------
TESTS
------------------------------------------------------------
*/

func testValidRequest() {
	const name = "John"
	url := baseURL() + "?name=" + name

	resp, body, err := get(url)
	if err != nil {
		fail("VALID_REQUEST", "HTTP failure", "no error", err.Error(), nil)
	}

	if resp.StatusCode != 200 {
		fail("VALID_REQUEST", "unexpected status code", "200", fmt.Sprintf("%d", resp.StatusCode), body)
	}

	r, err := decodeSuccess(body)
	if err != nil {
		fail("VALID_REQUEST", "invalid JSON schema", "decodeable SuccessResponse", err.Error(), body)
	}

	if r.Status != "success" {
		fail("VALID_REQUEST", "status field mismatch", "success", r.Status, body)
	}

	// Validate that data fields are actually populated — catches double-nested responses
	// or any response where the struct decoded but the fields zeroed out.
	if r.Data.Name == "" {
		fail("VALID_REQUEST", "data.name is empty", "non-empty string", `""`, body)
	}

	if r.Data.Gender != "male" && r.Data.Gender != "female" {
		fail("VALID_REQUEST", "data.gender is not a valid value", `"male" or "female"`, fmt.Sprintf("%q", r.Data.Gender), body)
	}

	if r.Data.Probability < 0.0 || r.Data.Probability > 1.0 {
		fail("VALID_REQUEST", "data.probability out of range", "0.0–1.0", fmt.Sprintf("%.4f", r.Data.Probability), body)
	}

	if r.Data.SampleSize <= 0 {
		fail("VALID_REQUEST", "data.sample_size is not positive", "> 0", fmt.Sprintf("%d", r.Data.SampleSize), body)
	}

	if r.Data.ProcessedAt == "" {
		fail("VALID_REQUEST", "data.processed_at is empty", "RFC3339 timestamp", `""`, body)
	}

	pass("VALID_REQUEST")
}

func testIsConfidentLogic() {
	url := baseURL() + "?name=John"

	resp, body, err := get(url)
	if err != nil {
		fail("CONFIDENCE_LOGIC", "HTTP failure", "no error", err.Error(), nil)
	}

	if resp.StatusCode != 200 {
		fail("CONFIDENCE_LOGIC", "unexpected status code", "200", fmt.Sprintf("%d", resp.StatusCode), body)
	}

	r, err := decodeSuccess(body)
	if err != nil {
		fail("CONFIDENCE_LOGIC", "invalid JSON", "decodeable SuccessResponse", err.Error(), body)
	}

	p := r.Data.Probability
	s := r.Data.SampleSize
	detail := fmt.Sprintf("probability=%.2f sample_size=%d", p, s)

	if r.Data.IsConfident {
		// is_confident=true requires p >= 0.7 AND sample_size >= 100
		if p < 0.7 || s < 100 {
			fail("CONFIDENCE_LOGIC",
				"is_confident=true but thresholds not met",
				"probability >= 0.7 AND sample_size >= 100",
				detail,
				body,
			)
		}
	} else {
		// is_confident=false requires p < 0.7 OR sample_size < 100
		if p >= 0.7 && s >= 100 {
			fail("CONFIDENCE_LOGIC",
				"is_confident=false but both thresholds are met",
				"probability < 0.7 OR sample_size < 100",
				detail,
				body,
			)
		}
	}

	pass("CONFIDENCE_LOGIC")
}

func testMissingName() {
	url := baseURL()

	resp, body, err := get(url)
	if err != nil {
		fail("MISSING_NAME", "HTTP failure", "no error", err.Error(), nil)
	}

	if resp.StatusCode != 400 {
		fail("MISSING_NAME", "unexpected status code", "400", fmt.Sprintf("%d", resp.StatusCode), body)
	}

	pass("MISSING_NAME")
}

func testInvalidName() {
	url := baseURL() + "?name=1234"

	resp, body, err := get(url)
	if err != nil {
		fail("INVALID_NAME", "HTTP failure", "no error", err.Error(), nil)
	}

	if resp.StatusCode != 422 {
		fail("INVALID_NAME", "unexpected status code", "422", fmt.Sprintf("%d", resp.StatusCode), body)
	}

	pass("INVALID_NAME")
}

func testEdgeCase() {
	url := baseURL() + "?name=asdkfjhasdkjfh"

	resp, body, err := get(url)
	if err != nil {
		fail("EDGE_CASE", "HTTP failure", "no error", err.Error(), nil)
	}

	if resp.StatusCode != 500 {
		fail("EDGE_CASE", "unexpected status code", "500", fmt.Sprintf("%d", resp.StatusCode), body)
	}

	r, err := decodeError(body)
	if err != nil {
		fail("EDGE_CASE", "invalid error JSON", "decodeable ErrorResponse", err.Error(), body)
	}

	if r.Status != "error" {
		fail("EDGE_CASE", "status field mismatch", "error", r.Status, body)
	}

	const wantMsg = "No prediction available for the provided name"
	if !strings.EqualFold(r.Message, wantMsg) {
		fail("EDGE_CASE", "unexpected error message", wantMsg, r.Message, body)
	}

	pass("EDGE_CASE")
}

func testTimestampFormat() {
	url := baseURL() + "?name=John"

	resp, body, err := get(url)
	if err != nil {
		fail("TIMESTAMP", "HTTP failure", "no error", err.Error(), nil)
	}

	if resp.StatusCode != 200 {
		fail("TIMESTAMP", "unexpected status code", "200", fmt.Sprintf("%d", resp.StatusCode), body)
	}

	r, err := decodeSuccess(body)
	if err != nil {
		fail("TIMESTAMP", "invalid JSON", "decodeable SuccessResponse", err.Error(), body)
	}

	if _, err := time.Parse(time.RFC3339, r.Data.ProcessedAt); err != nil {
		fail("TIMESTAMP", "invalid timestamp format", "RFC3339 (e.g. 2006-01-02T15:04:05Z)", r.Data.ProcessedAt, body)
	}

	pass("TIMESTAMP")
}

/*
------------------------------------------------------------
ENTRYPOINT
------------------------------------------------------------
*/

func main() {
	flag.Parse()

	fmt.Println("Running black-box API tests against:", baseURL())

	testValidRequest()
	testIsConfidentLogic()
	testMissingName()
	testInvalidName()
	testEdgeCase()
	testTimestampFormat()

	fmt.Println("\nALL TESTS PASSED")
}
