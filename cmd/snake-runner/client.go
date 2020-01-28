package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/reconquest/snake-runner/internal/requests"
	"github.com/reconquest/snake-runner/internal/responses"
	"github.com/reconquest/snake-runner/internal/sshkey"
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
	client.useragent = "snake-runner/" + version

	return client
}

func (client *Client) request() *Request {
	request := NewRequest(http.DefaultClient).
		BaseURL(client.baseURL).
		UserAgent(client.useragent).
		// required by bitbucket itself
		Header(NameHeader, client.config.Name).
		Header("X-Atlassian-Token", "no-check")

	if client.config.AccessToken != "" {
		request.Header(AccessTokenHeader, client.config.AccessToken)
	}

	return request
}

func (client *Client) Heartbeat(request *requests.Heartbeat) error {
	err := client.request().
		POST().Path("/gate/heartbeat").
		Payload(request).
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
	sshKey *sshkey.Key,
) (interface{}, error) {
	var response responses.Task

	err := client.request().
		POST().
		Path("/gate/task").
		Payload(requests.NewTask(runningPipelines, queryPipeline, sshKey.Public)).
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
