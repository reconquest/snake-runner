package main

type Pipeline struct {
	ID           int    `json:"id"`
	ProjectID    int    `json:"project_id"`
	RepositoryID int    `json:"repository_id"`
	Commit       string `json:"commit"`
	CreatedAt    int    `json:"created_at"`
	UpdatedAt    int    `json:"updated_at"`
	Status       string `json:"status"`
	Error        string `json:"error"`
	// ignore unused
	// JobsTotal int
	RunnerID int `json:"runner_id"`
}
