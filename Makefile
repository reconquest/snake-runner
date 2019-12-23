rr:
	rm -f conf/token conf/sshkey conf/sshkey.pub
	SNAKE_NAME=$$RANDOM gorun ./ -c ./conf/snake.dev.conf
