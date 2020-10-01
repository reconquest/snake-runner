package runner

import (
	"fmt"
	"os"
	"text/template"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/platform"
	"github.com/seletskiy/tplutil"
)

var templateInstalledButNotConfigured = template.Must(template.New("").Parse(`
The snake-runner has been successfully installed.
Now it needs to be connected to your Bitbucket Server to start running jobs.
To do so, provide the configuration parameters:

{{ if not .MasterAddress -}}
{{" "}}- The URL of Bitbucket Server with the Snake CI add-on installed.
{{ end -}}
{{- if not .RegistrationToken -}}
{{" "}}- The Registration Token, which can be found in the 'SNAKE CI/CD → Runners' section in Bitbucket Server Admin Panel.
{{ end }}

{{- if .IsDocker }}
Pass the URL of Bitbucket Server and Registration Token to docker the run command
used to start snake-runner:

 docker run \
    -e SNAKE_MASTER_ADDRESS={{ with .MasterAddress }}{{ . }}{{ else }}https://mybitbucket.company/{{ end }} \
    -e SNAKE_REGISTRATION_TOKEN={{ with .RegistrationToken }}{{ . }}{{ else }}<registration-token-here>{{ end }} \
    <other-docker-flags-here>
{{- else }}
Specify the URL of Bitbucket Server and Registration Token in the snake-runner
command:

 SNAKE_MASTER_ADDRESS={{ with .MasterAddress }}{{ . }}{{ else }}https://mybitbucket.company/{{ end }} \
 SNAKE_REGISTRATION_TOKEN={{ with .RegistrationToken }}{{ . }}{{ else }}<registration-token-here>{{ end }} \
    snake-runner

Alternatively, you can specify these params in the config file ` + DEFAULT_CONFIG_PATH + `:

 master_address: {{ with .MasterAddress }}{{ . }}{{ else }}https://mybitbucket.company/{{ end }}
 registration_token: {{ with .RegistrationToken }}{{ . }}{{ else }}<registration-token-here>{{ end }}
{{- end }}

Check our docs: https://snake-ci.com/docs/`))

var templateNotInstalledNotConfigured = template.Must(template.New("").Parse(`
Snake Runner is almost ready to handle the workload.
It needs to connect to your Bitbucket Server to start running jobs. 

To do so, provide the configuration parameters:

{{ if not .MasterAddress -}}
{{" "}}- The URL of Bitbucket Server with the Snake CI add-on installed.
{{ end -}}
{{- if not .RegistrationToken -}}
{{" "}}- The Registration Token, which can be found in the 'SNAKE CI/CD → Runners' section in Bitbucket Server Admin Panel.
{{- end }}

Specify the configuration parameters in the config file ` + DEFAULT_CONFIG_PATH + `:

 master_address: {{ with .MasterAddress }}{{ . }}{{ else }}https://mybitbucket.company/{{ end }}
 registration_token: {{ with .RegistrationToken }}{{ . }}{{ else }}<registration-token-here>{{ end }}

 {{- if .IsWindows }}

Add the following line in your config if you are not going to use Docker containers:
{{" "}}exec_mode: "shell"
 {{- end }}

Check our docs for more info: https://snake-ci.com/docs/`))

func ShowMessageInstalledButNotConfigured(config *Config) {
	message, err := tplutil.ExecuteToString(templateInstalledButNotConfigured, map[string]interface{}{
		"MasterAddress":     config.MasterAddress,
		"RegistrationToken": config.RegistrationToken,
		"IsDocker":          IsDocker(),
		"IsWindows":         platform.CURRENT == platform.WINDOWS,
	})
	if err != nil {
		log.Errorf(err, "unable to show templated message")

		fmt.Fprintf(
			os.Stderr,
			"SNAKE_MASTER_ADDRESS or SNAKE_REGISTRATION_TOKEN is not specified\n",
		)
		return
	}

	fmt.Fprintln(os.Stderr, message)
}

func ShowMessageNotInstalledNotConfigured(config *Config) {
	message, err := tplutil.ExecuteToString(templateNotInstalledNotConfigured, map[string]interface{}{
		"MasterAddress":     config.MasterAddress,
		"RegistrationToken": config.RegistrationToken,
		"IsWindows":         platform.CURRENT == platform.WINDOWS,
	})
	if err != nil {
		log.Errorf(err, "unable to show templated message")

		fmt.Fprintf(
			os.Stderr,
			"SNAKE_MASTER_ADDRESS or SNAKE_REGISTRATION_TOKEN is not specified\n",
		)
		return
	}

	fmt.Fprintln(os.Stderr, message)
}
