package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/reconquest/snake-runner/internal/requests"
	"github.com/reconquest/snake-runner/internal/responses"
	"github.com/reconquest/snake-runner/internal/tasks"
)

type Client struct {
	config    *RunnerConfig
	useragent string
	baseURL   string
}

func NewClient(config *RunnerConfig) *Client {
	client := &Client{}
	client.config = config

	master := strings.TrimSuffix(client.config.MasterAddress, "/")
	client.baseURL = master + MasterPrefixAPI

	return client
}

func (client *Client) request() *Request {
	return NewRequest(http.DefaultClient).
		BaseURL(client.baseURL).
		UserAgent("snake-runner/"+version).
		// required by bitbucket itself
		Header(NameHeader, client.config.Name).
		Header(TokenHeader, client.config.Token).
		Header("X-Atlassian-Token", "no-check")
}

func (client *Client) Heartbeat() error {
	err := client.request().
		POST().Path("/gate/heartbeat").
		Payload(requests.Heartbeat{}).
		Do()
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) Register(
	request requests.RunnerRegister,
) (responses.RunnerRegister, error) {
	var response responses.RunnerRegister
	err := client.request().
		POST().Path("/gate/register").
		Payload(request).
		Response(&response).
		Do()
	return response, err
}

func (client *Client) GetTask(
	runningPipelines []int,
	queryPipeline bool,
) (interface{}, error) {
	var response responses.Task

	payload := struct {
		RunningPipelines []int `json:"running_pipelines"`
		QueryPipeline    bool  `json:"query_pipeline"`
	}{
		RunningPipelines: runningPipelines,
		QueryPipeline:    queryPipeline,
	}

	err := client.request().
		POST().
		Path("/gate/task").
		Payload(payload).
		Response(&response).
		Do()
	if err != nil {
		return nil, err
	}

	return tasks.Unmarshal(response)
}

func (client *Client) UpdatePipeline(
	id int,
	status string,
	startedAt *time.Time,
	finishedAt *time.Time,
) error {
	request := requests.TaskUpdate{
		Status:     status,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}

	return client.request().
		PUT().
		Path("/gate/pipelines/" + strconv.Itoa(id)).
		Payload(request).
		Do()
}

func (client *Client) UpdateJob(
	pipelineID int,
	jobID int,
	status string,
	startedAt *time.Time,
	finishedAt *time.Time,
) error {
	request := requests.TaskUpdate{
		Status:     status,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}

	return client.request().
		PUT().
		Path(
			"/gate" +
				"/pipelines/" + strconv.Itoa(pipelineID) +
				"/jobs/" + strconv.Itoa(jobID),
		).
		Payload(request).
		Do()
}

func (client *Client) PushLogs(pipelineID int, jobID int, text string) error {
	return client.request().
		POST().
		Path(
			"/gate/pipelines/" + strconv.Itoa(pipelineID) +
				"/jobs/" + strconv.Itoa(jobID) +
				"/logs",
		).
		Payload(&requests.LogsPush{
			Data: text,
		}).
		Do()
}
