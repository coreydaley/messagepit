package mailevents

import (
	"encoding/json"
	"strings"
)

// SMTPAPIHeader is the header real SendGrid uses to carry categories and
// custom_args through an SMTP hop. MessagePit reuses it so the SendGrid v3
// handler can hand that metadata to the storage layer without a side channel.
const SMTPAPIHeader = "X-SMTPAPI"

// ScenarioHeader is a MessagePit extension: it pins the lifecycle branch
// explicitly instead of deriving it from the recipient address.
const ScenarioHeader = "X-MessagePit-Scenario"

// smtpAPI is the subset of the X-SMTPAPI payload MessagePit understands.
type smtpAPI struct {
	Category   []string          `json:"category,omitempty"`
	UniqueArgs map[string]string `json:"unique_args,omitempty"`
}

// BuildSMTPAPI renders an X-SMTPAPI header value. Returns "" when there is
// nothing to carry, so callers can skip writing the header entirely.
func BuildSMTPAPI(categories []string, uniqueArgs map[string]string) string {
	if len(categories) == 0 && len(uniqueArgs) == 0 {
		return ""
	}

	b, err := json.Marshal(smtpAPI{Category: categories, UniqueArgs: uniqueArgs})
	if err != nil {
		return ""
	}

	// Header values must stay on one line; JSON encoding already escapes
	// CR/LF inside strings, but be explicit about the invariant.
	return strings.NewReplacer("\r", "", "\n", "").Replace(string(b))
}

// ParseSMTPAPI extracts categories and custom_args from an X-SMTPAPI value.
// A malformed value yields empty results rather than an error — the header is
// advisory metadata, not something worth failing a capture over.
func ParseSMTPAPI(value string) ([]string, map[string]string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	var parsed smtpAPI
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return nil, nil
	}

	return parsed.Category, parsed.UniqueArgs
}
