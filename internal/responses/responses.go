package responses

import (
	"encoding/json"
	"fmt"
)

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

type PullRequest struct {
	ID          int            `json:"id"`
	Title       string         `json:"title"`
	State       string         `json:"state"`
	FromRef     PullRequestRef `json:"from_ref"`
	ToRef       PullRequestRef `json:"to_ref"`
	IsCrossRepo bool           `json:"is_cross_repo"`
}

type PullRequestRef struct {
	Hash   string `json:"hash"`
	Ref    string `json:"ref"`
	IsFork bool   `json:"is_fork"`
}

type KnownHost struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Key      string `json:"key"`
}

func (host *KnownHost) HostPort() string {
	if host.Port != 22 && host.Port != 0 {
		return "[" + host.Hostname + "]:" + fmt.Sprint(host.Port)
	}

	return host.Hostname
}
