package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

type remoteError struct {
	ErrorMessage string `json:"error"`
}

func (error remoteError) Error() string { return error.ErrorMessage }

type Request struct {
	httpClient *http.Client

	baseURL string
	method  string
	path    string

	hasPayload bool
	payload    interface{}

	expectedStatuses []int
	dstResponse      interface{}

	headers map[string]string
}

func NewRequest(client *http.Client) *Request {
	request := &Request{}
	request.httpClient = client
	request.headers = map[string]string{}
	return request
}

func (request *Request) BaseURL(url string) *Request {
	request.baseURL = url
	return request
}

func (request *Request) PUT() *Request {
	return request.Method("PUT")
}

func (request *Request) GET() *Request {
	return request.Method("GET")
}

func (request *Request) POST() *Request {
	return request.Method("POST")
}

func (request *Request) Method(name string) *Request {
	request.method = name
	return request
}

func (request *Request) UserAgent(useragent string) *Request {
	return request.Header("User-Agent", useragent)
}

func (request *Request) Header(name string, value string) *Request {
	request.headers[name] = value
	return request
}

func (request *Request) Payload(body interface{}) *Request {
	request.hasPayload = true
	request.payload = body
	return request
}

func (request *Request) Response(response interface{}) *Request {
	request.dstResponse = response
	return request
}

func (request *Request) ExpectStatus(code ...int) *Request {
	request.expectedStatuses = code
	return request
}

func (request *Request) Path(url string) *Request {
	request.path = url
	return request
}

func (request *Request) setContentTypeJSON() {
	request.headers["Content-Type"] = "application/json"
}

func (request *Request) Do() error {
	var httpRequest *http.Request
	var err error

	if request.method == "" {
		request.method = "GET"
	}

	url := request.getURL()

	context := karma.Describe("method", request.method).
		Describe("url", url)

	if request.hasPayload {
		// currently we assume that the payload should be JSON encoded
		request.setContentTypeJSON()

		buffer := bytes.NewBuffer(nil)
		err := json.NewEncoder(buffer).Encode(request.payload)
		if err != nil {
			return karma.Format(
				err,
				"unable to marshal request body",
			)
		}

		context = context.Describe(
			"payload",
			strings.TrimSpace(buffer.String()),
		)

		httpRequest, err = http.NewRequest(request.method, url, buffer)
	} else {
		httpRequest, err = http.NewRequest(request.method, url, nil)
	}
	if err != nil {
		return context.Format(
			err,
			"unable to create http request",
		)
	}

	debugContext := context
	for key, value := range request.headers {
		debugContext = debugContext.Describe("header "+key, value)
		httpRequest.Header.Set(key, value)
	}

	log.Tracef(debugContext, "sending http request")

	httpResponse, err := request.httpClient.Do(httpRequest)
	if err != nil {
		return context.Format(
			err,
			"unable to make http request",
		)
	}

	data, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return context.Format(
			err,
			"unable to read response body",
		)
	}

	defer httpResponse.Body.Close()

	log.Tracef(
		context.Describe("status_code", httpResponse.StatusCode),
		"response: %s",
		string(data),
	)

	expectedStatus := false
	for _, expected := range request.expectedStatuses {
		if httpResponse.StatusCode == expected {
			expectedStatus = true
			break
		}
	}

	if !expectedStatus {
		context = context.Describe("status_code", httpResponse.StatusCode)
		if httpResponse.StatusCode >= 400 {
			var errResponse remoteError
			if err := json.Unmarshal(data, &errResponse); err == nil {
				return context.Reason(errResponse)
			} else {
				return context.Describe("body", string(data)).
					Format(
						err,
						"unable to unmarshal error as JSON error response",
					)
			}
		} else if len(request.expectedStatuses) > 0 {
			return context.Reason("unexpected status code")
		}
	}

	if request.dstResponse != nil {
		err = json.Unmarshal(data, request.dstResponse)
		if err != nil {
			return context.Describe("body", string(data)).
				Format(
					err,
					"unable to unmarshal JSON response",
				)
		}
	}

	return nil
}

func (request *Request) getURL() string {
	address := request.baseURL
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}

	return strings.TrimSuffix(address, "/") + request.path
}
