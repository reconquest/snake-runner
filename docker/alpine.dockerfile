FROM alpine:3

COPY /snake-runner.docker /bin/snake-runner

RUN  mkdir /etc/snake-runner/
COPY /conf/snake-runner.conf /etc/snake-runner/snake-runner.conf

ENV SNAKE_MASTER_ADDRESS="" SNAKE_LOG_DEBUG="" SNAKE_LOG_TRACE="" SNAKE_NAME=""\
 SNAKE_TOKEN="" SNAKE_TOKEN_PATH="" SNAKE_VIRTUALIZATION=""\
 SNAKE_MAX_PARALLEL_PIPELINES="" SNAKE_SSH_KEY_PATH=""

VOLUME /var/run/docker.sock /var/lib/snake-runner/secrets/

CMD ["/bin/snake-runner"]

# vim: ft=dockerfile
