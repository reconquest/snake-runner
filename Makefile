rr:
	rm conf/token
	SNAKE_NAME=$$RANDOM gorun ./ -c ./conf/snake.dev.conf
