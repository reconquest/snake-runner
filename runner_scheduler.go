package main

func (runner *Runner) startScheduler() {
	//
}

func (runner *Runner) getTask() {
	var response interface{}

	runner.request().GET().Path("/gate/task").Response(&response).Do()
}
