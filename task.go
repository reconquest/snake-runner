package main

type Task struct {
	Pipeline Pipeline      `json:"pipeline"`
	Jobs     []PipelineJob `json:"jobs"`
	CloneURL struct {
		SSH string `json:"ssh"`
	} `json:"clone_url"`
}

func (runner *Runner) getTask() (*Task, error) {
	var response Task

	err := runner.request().
		GET().
		Path("/gate/task").
		Response(&response).
		Do()
	if err != nil {
		return nil, err
	}

	//
	// response.CloneURL.SSH = strings.ReplaceAll(response.CloneURL.SSH, "localhost:7990", "bitbucket")

	return &response, nil
}
