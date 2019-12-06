package main

func (runner *Runner) startScheduler() {
	//
}

func (runner *Runner) getTask() {
	runner.request().GET().Path("/gate/task").Response(&response).Do()
}
