package responses

import "encoding/json"

type RunnerRegister struct {
	AccessToken string `json:"access_token"`
}

type Task struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

type Project struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

type Repository struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}
