package snake

type PipelineJob struct {
	ID           int    `json:"id"`
	PipelineID   int    `json:"pipeline_id"`
	ProjectID    int    `json:"project_id"`
	RepositoryID int    `json:"repository_id"`
	Commit       string `json:"commit"`
	CreatedAt    int    `json:"created_at"`
	UpdatedAt    int    `json:"updated_at"`
	Stage        string `json:"stage"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	RunnerID     int    `json:"runner_id"`
}
