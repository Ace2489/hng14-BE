# HNG14-BE (Stage 0 & 1)

A RESTful API built with Go that classifies names using the Genderize API and manages user profiles.

## Tech Stack

* Go.
* Sqlite.
* That's it. Life doesn't need to be complicated.
---

## Getting Started

### Prerequisites

* Go 1.26 or higher

### Clone the Repository

```bash
git clone https://github.com/Ace2489/hng14-BE
cd hng14-BE
```

### Run the Application

```bash
LOG_LEVEL=<LEVEL> PORT=<PORT> go run ./src
```

Where:

* `LEVEL` ∈ `DEBUG`, `INFO`, `WARN`, `ERROR` (default: INFO)
* `PORT` is the server port (e.g., 3000)

Example:

```bash
LOG_LEVEL=DEBUG PORT=3000 go run ./src
```

---

## Running Tests

In a separate terminal:

```bash
go run apitests/main.go
```

### Test Results

```
--- Stage 0: /api/classify ---
[PASS] VALID_REQUEST
[PASS] CONFIDENCE_LOGIC
[PASS] MISSING_NAME
[PASS] INVALID_NAME
[PASS] EDGE_CASE
[PASS] TIMESTAMP

--- Stage 1: /api/profiles ---
[PASS] CREATE_PROFILE
[PASS] IDEMPOTENCY
[PASS] GET_PROFILE
[PASS] GET_ALL_PROFILES
[PASS] FILTER_GENDER_CASE
[PASS] FILTER_COUNTRY
[PASS] FILTER_AGE_GROUP
[PASS] FILTER_COMBINED
[PASS] PROFILE_NOT_FOUND
[PASS] PROFILE_MISSING_NAME
[PASS] PROFILE_EMPTY_NAME
[PASS] PROFILE_INVALID_NAME
[PASS] CORS_HEADER
[PASS] DELETE_PROFILE
[PASS] DELETED_PROFILE_GONE
[PASS] DELETE_NOT_FOUND

ALL TESTS PASSED
```
