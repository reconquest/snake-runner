package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/reconquest/pkg/log"
	"github.com/seletskiy/tplutil"
)

var templateNotConfigured = template.Must(template.New("").Parse(`
The snake-runner was successfully installed.
Now it needs to be connected to your Bitbucket Server to start running jobs.
To do so, provide configuration parameters:

{{ if not .MasterAddress -}}
{{" "}}- URL of Bitbucket Server with Snake CI add-on installed.
{{ end -}}
{{- if not .RegistrationToken -}}
{{" "}}- Registration Token, which can be found in 'CI/CD → Runners' section in
   Bitbucket Server Admin Panel.
{{ end }}

{{- if .IsDocker }}
Pass URL of Bitbucket Server and Registration Token to docker run command
used to start snake-runner:

 docker run \
    -e SNAKE_MASTER_ADDRESS={{ with .MasterAddress }}{{ . }}{{ else }}https://mybitbucket.company/{{ end }} \
    -e SNAKE_REGISTRATION_TOKEN={{ with .RegistrationToken }}{{ . }}{{ else }}<registration-token-here>{{ end }} \
    <other-docker-flags-here>
{{- else }}
Specify URL of Bitbucket Server and Registration Token in snake-runner
command:

 SNAKE_MASTER_ADDRESS={{ with .MasterAddress }}{{ . }}{{ else }}https://mybitbucket.company/{{ end }} \
 SNAKE_REGISTRATION_TOKEN={{ with .RegistrationToken }}{{ . }}{{ else }}<registration-token-here>{{ end }} \
    snake-runner

Alternatively, you can specify those params in the config file, which by
default is located in /etc/snake-runner/snake-runner.conf:

 master_address: {{ with .MasterAddress }}{{ . }}{{ else }}https://mybitbucket.company/{{ end }}
 registration_token: {{ with .RegistrationToken }}{{ . }}{{ else }}<registration-token-here>{{ end }}
{{- end }}
`))

func ShowMessageNotConfigured(config RunnerConfig) {
	message, err := tplutil.ExecuteToString(templateNotConfigured, map[string]interface{}{
		"MasterAddress":     config.MasterAddress,
		"RegistrationToken": config.RegistrationToken,
		"IsDocker":          isDocker(),
	})
	if err != nil {
		log.Errorf(err, "unable to show templated message")

		fmt.Fprintf(os.Stderr, "SNAKE_MASTER_ADDRESS or SNAKE_REGISTRATION_TOKEN is not specified\n")
		return
	}

	fmt.Fprintln(os.Stderr, message)
}

func isDocker() bool {
	contents, err := ioutil.ReadFile("/proc/1/cgroup")
	if err != nil {
		log.Errorf(err, "unable to read /proc/1/cgroup to determine "+
			"is it docker container or not")
	}

	/**
	* A docker container has /docker/ in its /cgroup file
	*
	* / # cat /proc/1/cgroup | grep docker
	* 11:pids:/docker/14f3db3a669169c0b801a3ac99...
	* 10:freezer:/docker/14f3db3a669169c0b801a3ac9...
	* 9:cpu,cpuacct:/docker/14f3db3a669169c0b801a3ac...
	* 8:hugetlb:/docker/14f3db3a669169c0b801a3ac99f89e...
	* 7:perf_event:/docker/14f3db3a669169c0b801a3...
	* 6:devices:/docker/14f3db3a669169c0b801a3ac99f...
	* 5:memory:/docker/14f3db3a669169c0b801a3ac99f89e...
	* 4:blkio:/docker/14f3db3a669169c0b801a3ac99f89e914...
	* 3:cpuset:/docker/14f3db3a669169c0b801a3ac99f89e914a...
	* 2:net_cls,net_prio:/docker/14f3db3a669169c0b801a3ac...
	* 1:name=systemd:/docker/14f3db3a669169c0b801a3ac99f89e...
	* 0::/system.slice/docker.service
	***/
	if strings.Contains(string(contents), "/docker/") {
		return true
	}

	return false
}
