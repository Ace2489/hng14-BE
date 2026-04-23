# HNG14-BE 
The slop generator cometh!

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

# Natural Language Query Parser

## How It Works

`GET /api/profiles/search?q=` accepts a plain English query and converts it into structured filters before hitting the same query path as `GET /api/profiles`.

## Supported Keywords

**Gender**
| Query contains | Maps to |
|---|---|
| `female` | `gender=female` |
| `male` | `gender=male` |

`female` is checked first, so "male and female" resolves to `female`. Explicit phrasing like "males and females" won't yield both, so you have to exclude the filter if you want all of it.

**Age Groups**
| Keyword | Maps to |
|---|---|
| `child` | `age_group=child` |
| `teenager` | `age_group=teenager` |
| `adult` | `age_group=adult` |
| `senior` | `age_group=senior` |
| `young` | `min_age=16` + `max_age=24` |

**Age Modifiers**

Looks for a keyword immediately followed by a number:

| Pattern | Maps to |
|---|---|
| `above N` / `over N` | `min_age=N` |
| `below N` / `under N` | `max_age=N` |

**Country**

Matches the phrase after `from` to ISO codes. Multi-word countries (e.g. `ivory coast`, `south africa`) are matched by trying progressively shorter suffixes.

---

## Examples

```
young males from nigeria        →  gender=male, min_age=16, max_age=24, country_id=NG
females above 30                →  gender=female, min_age=30
adult males from kenya          →  gender=male, age_group=adult, country_id=KE
teenagers below 19              →  age_group=teenager, max_age=19
people from ivory coast         →  country_id=CI
```

Queries that produce zero filters return `400 Unable to interpret query`.

---

## Limitations

**Gender is not additive.** "males and females" does not return both — it resolves to `female` because that keyword is matched first. Queries intending both genders should omit gender entirely.

**`young` and `age_group` can conflict.** "young adults" sets both `min_age=16&max_age=24` and `age_group=adult`. The DB will apply both constraints, which may return fewer results than expected.

**Age modifiers require the number to be the next token.** "above thirty" won't parse — only digits work. "ages above 30" won't work either — the word before the number must be the modifier keyword directly.

**Country matching is a fixed list.** Countries not in `countryMap` are silently ignored. The query still runs if other filters matched; it only returns `Unable to interpret query` if *nothing* parsed.

**No negation.** "not from nigeria", "excluding seniors", "non-binary" — none of these are handled.

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
