package main

type RunnerTask struct {
	Pipeline *Pipeline
	Jobs     *[]PipelineJob
}

func (runner *Runner) getTask() (*RunnerTask, error) {
	var response RunnerTask

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
