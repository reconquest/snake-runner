package snake

type Pipeline struct {
	ID           int    `json:"id"`
	ProjectID    int    `json:"project_id"`
	RepositoryID int    `json:"repository_id"`
	Commit       string `json:"commit"`
	FromCommit   string `json:"from_commit"`
	CreatedAt    int    `json:"created_at"`
	UpdatedAt    int    `json:"updated_at"`
	Status       string `json:"status"`
	Error        string `json:"error"`
	Filename     string `json:"filename"`
	// ignore unused
	// JobsTotal int
	RunnerID      int    `json:"runner_id"`
	RefType       string `json:"ref_type"`
	RefDisplayId  string `json:"ref_display_id"`
	PullRequestID int    `json:"pull_request_id"`
}
