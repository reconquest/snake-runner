package responses

import "encoding/json"

type RunnerRegister struct {
	AuthenticationToken string `json:"authentication_token"`
}

type Task struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}
