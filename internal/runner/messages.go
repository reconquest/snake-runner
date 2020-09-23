package runner

import (
	"fmt"
	"os"
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
default is ` + DEFAULT_PIPELINES_DIR + `:

 master_address: {{ with .MasterAddress }}{{ . }}{{ else }}https://mybitbucket.company/{{ end }}
 registration_token: {{ with .RegistrationToken }}{{ . }}{{ else }}<registration-token-here>{{ end }}
{{- end }}
`))

func ShowMessageNotConfigured(config Config) {
	message, err := tplutil.ExecuteToString(templateNotConfigured, map[string]interface{}{
		"MasterAddress":     config.MasterAddress,
		"RegistrationToken": config.RegistrationToken,
		"IsDocker":          IsDocker(),
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
