package providers

import (
	"encoding/base64"
	"encoding/json"
)

type SearchPayload struct {
	Method      string        `json:"method"`
	CallbackURL string        `json:"callback_url"`
	Args        []interface{} `json:"args"`
}

func (sp *SearchPayload) String() string {
	b, err := json.Marshal(sp)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}
