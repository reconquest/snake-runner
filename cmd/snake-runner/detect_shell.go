package main

const DETECT_SHELL_COMMAND = `IFS=:;
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
