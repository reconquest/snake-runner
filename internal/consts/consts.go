package consts

const (
	DEFAULT_CONTAINER_JOB_IMAGE = "alpine:latest"

	DETECT_SHELL_COMMAND = `IFS=:;
if [ -z "$PATH" ]; then
	set -- $(getconf PATH)
else 
	set -- $PATH;
fi;
for dir; do
	if [ -x $dir/bash ]; then
		echo $dir/bash;
		exit 0;
	fi;
done;`

	SSH_CONFIG_NO_STRICT_HOST_KEY_CHECKING = `Host *
	StrictHostKeyChecking no
	UserKnownHostsFile /dev/null
`
)
