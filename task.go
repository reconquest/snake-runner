package main

type Task struct {
	Pipeline Pipeline
	Jobs     []PipelineJob
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

	return &response, nil
}
