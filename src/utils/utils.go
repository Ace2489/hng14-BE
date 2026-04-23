package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"unicode"
)

type Payload struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type LoggerKey struct{}

// Thin wrapper to enable formatting without requiring fmt.Sprintf all the time
type Logger struct {
	*slog.Logger
}

func (logger *Logger) DebugFmt(str string, args ...any) {
	logger.Debug(fmt.Sprintf(str, args...))
}

func (logger *Logger) InfoFmt(str string, args ...any) {
	logger.Info(fmt.Sprintf(str, args...))

}

func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") //REDUNDANT: This is now handled by the middleware
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, Payload{Status: "error", Message: message})
}

func WriteSuccess(w http.ResponseWriter, status int, data interface{}) {
	WriteSuccessWithMessage(w, status, "", data)
}
func WriteSuccessWithMessage(w http.ResponseWriter, status int, message string, data interface{}) {
	WriteJSON(w, status, Payload{Status: "success", Message: message, Data: data})
}

func WriteList(w http.ResponseWriter, status int, count int, data interface{}) {
	type listPayload struct {
		Status string      `json:"status"`
		Count  int         `json:"count"`
		Data   interface{} `json:"data"`
	}
	WriteJSON(w, status, listPayload{Status: "success", Count: count, Data: data})
}

func WritePaginatedResponse(w http.ResponseWriter, status int, page int, limit int, total int, data any) {
	type PaginatedResponse struct {
		Status string      `json:"status"`
		Page   int         `json:"page"`
		Limit  int         `json:"limit"`
		Total  int         `json:"total"`
		Data   interface{} `json:"data"`
	}
	WriteJSON(w, status, PaginatedResponse{Status: "success", Page: page, Limit: limit, Total: total, Data: data})
}

func LoggerFromCtx(ctx context.Context) Logger {
	if logger, ok := ctx.Value(LoggerKey{}).(Logger); ok {
		return logger
	}
	log.Println("No logger found. Falling back to the default logger")
	return Logger{Logger: slog.Default()}
}

func EnsureAlphabets(input string) bool {
	for _, c := range input {
		if !unicode.IsLetter(c) {
			return false
		}
	}
	return true
}

// Country code from Names
// Source: https://github.com/flaticols/countrycodes

// Source:https://github.com/flaticols/countrycodes/blob/e2262a47f4492a497035da4793349417591f2eab/names_generated.go

// Alpha2ToName retrieves the country name for an ISO 3166-1 alpha-2 code.
// This function performs an O(1) lookup using packed keys for zero-allocation,
// case-insensitive matching. Use this for displaying human-readable country names.
// The input is case-insensitive.
//
// Parameters:
//   - code: ISO 3166-1 alpha-2 code (e.g., "FR", "de", "IT", "es")
//
// Returns:
//   - string: The official English short name from ISO 3166-1
//   - bool: true if the code exists, false otherwise
//
// Example:
//
//	if name, ok := Alpha2ToName("fr"); ok {
//	    fmt.Println(name) // Output: France
//	}
//
// Note: Country names follow the official ISO 3166-1 English short names.
func Alpha2ToName(code string) (string, bool) {
	if len(code) != 2 {
		return "", false
	}
	// Inline key generation for better performance
	key := uint16(lowerASCII(code[0])) | uint16(lowerASCII(code[1]))<<8
	v, ok := alpha2ToNameMap[key]
	return v, ok
}

// Maps packed 2-char keys to country names
var alpha2ToNameMap = map[uint16]string{
	0x6461: "Andorra",
	0x6561: "United Arab Emirates",
	0x6661: "Afghanistan",
	0x6761: "Antigua and Barbuda",
	0x6961: "Anguilla",
	0x6c61: "Albania",
	0x6d61: "Armenia",
	0x6f61: "Angola",
	0x7161: "Antarctica",
	0x7261: "Argentina",
	0x7361: "American Samoa",
	0x7461: "Austria",
	0x7561: "Australia",
	0x7761: "Aruba",
	0x7861: "Åland Islands",
	0x7a61: "Azerbaijan",
	0x6162: "Bosnia and Herzegovina",
	0x6262: "Barbados",
	0x6462: "Bangladesh",
	0x6562: "Belgium",
	0x6662: "Burkina Faso",
	0x6762: "Bulgaria",
	0x6862: "Bahrain",
	0x6962: "Burundi",
	0x6a62: "Benin",
	0x6c62: "Saint Barthélemy",
	0x6d62: "Bermuda",
	0x6e62: "Brunei Darussalam",
	0x6f62: "Bolivia, Plurinational State of",
	0x7162: "Bonaire, Sint Eustatius and Saba",
	0x7262: "Brazil",
	0x7362: "Bahamas",
	0x7462: "Bhutan",
	0x7662: "Bouvet Island",
	0x7762: "Botswana",
	0x7962: "Belarus",
	0x7a62: "Belize",
	0x6163: "Canada",
	0x6363: "Cocos (Keeling) Islands",
	0x6463: "Congo, Democratic Republic of the",
	0x6663: "Central African Republic",
	0x6763: "Congo",
	0x6863: "Switzerland",
	0x6963: "Côte d'Ivoire",
	0x6b63: "Cook Islands",
	0x6c63: "Chile",
	0x6d63: "Cameroon",
	0x6e63: "China",
	0x6f63: "Colombia",
	0x7263: "Costa Rica",
	0x7563: "Cuba",
	0x7663: "Cabo Verde",
	0x7763: "Curaçao",
	0x7863: "Christmas Island",
	0x7963: "Cyprus",
	0x7a63: "Czechia",
	0x6564: "Germany",
	0x6a64: "Djibouti",
	0x6b64: "Denmark",
	0x6d64: "Dominica",
	0x6f64: "Dominican Republic",
	0x7a64: "Algeria",
	0x6365: "Ecuador",
	0x6565: "Estonia",
	0x6765: "Egypt",
	0x6865: "Western Sahara",
	0x7265: "Eritrea",
	0x7365: "Spain",
	0x7465: "Ethiopia",
	0x6966: "Finland",
	0x6a66: "Fiji",
	0x6b66: "Falkland Islands (Malvinas)",
	0x6d66: "Micronesia, Federated States of",
	0x6f66: "Faroe Islands",
	0x7266: "France",
	0x6167: "Gabon",
	0x6267: "United Kingdom of Great Britain and Northern Ireland",
	0x6467: "Grenada",
	0x6567: "Georgia",
	0x6667: "French Guiana",
	0x6767: "Guernsey",
	0x6867: "Ghana",
	0x6967: "Gibraltar",
	0x6c67: "Greenland",
	0x6d67: "Gambia",
	0x6e67: "Guinea",
	0x7067: "Guadeloupe",
	0x7167: "Equatorial Guinea",
	0x7267: "Greece",
	0x7367: "South Georgia and the South Sandwich Islands",
	0x7467: "Guatemala",
	0x7567: "Guam",
	0x7767: "Guinea-Bissau",
	0x7967: "Guyana",
	0x6b68: "Hong Kong",
	0x6d68: "Heard Island and McDonald Islands",
	0x6e68: "Honduras",
	0x7268: "Croatia",
	0x7468: "Haiti",
	0x7568: "Hungary",
	0x6469: "Indonesia",
	0x6569: "Ireland",
	0x6c69: "Israel",
	0x6d69: "Isle of Man",
	0x6e69: "India",
	0x6f69: "British Indian Ocean Territory",
	0x7169: "Iraq",
	0x7269: "Iran, Islamic Republic of",
	0x7369: "Iceland",
	0x7469: "Italy",
	0x656a: "Jersey",
	0x6d6a: "Jamaica",
	0x6f6a: "Jordan",
	0x706a: "Japan",
	0x656b: "Kenya",
	0x676b: "Kyrgyzstan",
	0x686b: "Cambodia",
	0x696b: "Kiribati",
	0x6d6b: "Comoros",
	0x6e6b: "Saint Kitts and Nevis",
	0x706b: "Korea, Democratic People's Republic of",
	0x726b: "Korea, Republic of",
	0x776b: "Kuwait",
	0x796b: "Cayman Islands",
	0x7a6b: "Kazakhstan",
	0x616c: "Lao People's Democratic Republic",
	0x626c: "Lebanon",
	0x636c: "Saint Lucia",
	0x696c: "Liechtenstein",
	0x6b6c: "Sri Lanka",
	0x726c: "Liberia",
	0x736c: "Lesotho",
	0x746c: "Lithuania",
	0x756c: "Luxembourg",
	0x766c: "Latvia",
	0x796c: "Libya",
	0x616d: "Morocco",
	0x636d: "Monaco",
	0x646d: "Moldova, Republic of",
	0x656d: "Montenegro",
	0x666d: "Saint Martin (French part)",
	0x676d: "Madagascar",
	0x686d: "Marshall Islands",
	0x6b6d: "North Macedonia",
	0x6c6d: "Mali",
	0x6d6d: "Myanmar",
	0x6e6d: "Mongolia",
	0x6f6d: "Macao",
	0x706d: "Northern Mariana Islands",
	0x716d: "Martinique",
	0x726d: "Mauritania",
	0x736d: "Montserrat",
	0x746d: "Malta",
	0x756d: "Mauritius",
	0x766d: "Maldives",
	0x776d: "Malawi",
	0x786d: "Mexico",
	0x796d: "Malaysia",
	0x7a6d: "Mozambique",
	0x616e: "Namibia",
	0x636e: "New Caledonia",
	0x656e: "Niger",
	0x666e: "Norfolk Island",
	0x676e: "Nigeria",
	0x696e: "Nicaragua",
	0x6c6e: "Netherlands, Kingdom of the",
	0x6f6e: "Norway",
	0x706e: "Nepal",
	0x726e: "Nauru",
	0x756e: "Niue",
	0x7a6e: "New Zealand",
	0x6d6f: "Oman",
	0x6170: "Panama",
	0x6570: "Peru",
	0x6670: "French Polynesia",
	0x6770: "Papua New Guinea",
	0x6870: "Philippines",
	0x6b70: "Pakistan",
	0x6c70: "Poland",
	0x6d70: "Saint Pierre and Miquelon",
	0x6e70: "Pitcairn",
	0x7270: "Puerto Rico",
	0x7370: "Palestine, State of",
	0x7470: "Portugal",
	0x7770: "Palau",
	0x7970: "Paraguay",
	0x6171: "Qatar",
	0x6572: "Réunion",
	0x6f72: "Romania",
	0x7372: "Serbia",
	0x7572: "Russian Federation",
	0x7772: "Rwanda",
	0x6173: "Saudi Arabia",
	0x6273: "Solomon Islands",
	0x6373: "Seychelles",
	0x6473: "Sudan",
	0x6573: "Sweden",
	0x6773: "Singapore",
	0x6873: "Saint Helena, Ascension and Tristan da Cunha",
	0x6973: "Slovenia",
	0x6a73: "Svalbard and Jan Mayen",
	0x6b73: "Slovakia",
	0x6c73: "Sierra Leone",
	0x6d73: "San Marino",
	0x6e73: "Senegal",
	0x6f73: "Somalia",
	0x7273: "Suriname",
	0x7373: "South Sudan",
	0x7473: "Sao Tome and Principe",
	0x7673: "El Salvador",
	0x7873: "Sint Maarten (Dutch part)",
	0x7973: "Syrian Arab Republic",
	0x7a73: "Eswatini",
	0x6374: "Turks and Caicos Islands",
	0x6474: "Chad",
	0x6674: "French Southern Territories",
	0x6774: "Togo",
	0x6874: "Thailand",
	0x6a74: "Tajikistan",
	0x6b74: "Tokelau",
	0x6c74: "Timor-Leste",
	0x6d74: "Turkmenistan",
	0x6e74: "Tunisia",
	0x6f74: "Tonga",
	0x7274: "Türkiye",
	0x7474: "Trinidad and Tobago",
	0x7674: "Tuvalu",
	0x7774: "Taiwan, Province of China",
	0x7a74: "Tanzania, United Republic of",
	0x6175: "Ukraine",
	0x6775: "Uganda",
	0x6d75: "United States Minor Outlying Islands",
	0x7375: "United States of America",
	0x7975: "Uruguay",
	0x7a75: "Uzbekistan",
	0x6176: "Holy See",
	0x6376: "Saint Vincent and the Grenadines",
	0x6576: "Venezuela, Bolivarian Republic of",
	0x6776: "Virgin Islands (British)",
	0x6976: "Virgin Islands (U.S.)",
	0x6e76: "Viet Nam",
	0x7576: "Vanuatu",
	0x6677: "Wallis and Futuna",
	0x7377: "Samoa",
	0x6579: "Yemen",
	0x7479: "Mayotte",
	0x617a: "South Africa",
	0x6d7a: "Zambia",
	0x777a: "Zimbabwe",
}

// Source: https://github.com/flaticols/countrycodes/blob/e2262a47f4492a497035da4793349417591f2eab/optimizations.go
// lowerASCII converts an ASCII uppercase letter to lowercase without allocations.
// Non-letter bytes are returned unchanged.
//
//go:inline
func lowerASCII(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}
