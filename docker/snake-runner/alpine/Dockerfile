FROM alpine:3

COPY /snake-runner.docker /bin/snake-runner

RUN  mkdir /etc/snake-runner/
COPY /conf/snake-runner.conf /etc/snake-runner/snake-runner.conf

ENV SNAKE_MASTER_ADDRESS="" \
    SNAKE_LOG_DEBUG="" \
    SNAKE_LOG_TRACE="" \
    SNAKE_NAME="" \
    SNAKE_REGISTRATION_TOKEN="" \
    SNAKE_ACCESS_TOKEN="" \
    SNAKE_ACCESS_TOKEN_PATH="" \
    SNAKE_HEARTBEAT_INTERVAL="" \
    SNAKE_SCHEDULER_INTERVAL="" \
    SNAKE_VIRTUALIZATION="" \
    SNAKE_MAX_PARALLEL_PIPELINES="" \
    SNAKE_PIPELINES_DIR="" \
    SNAKE_DOCKER_NETWORK="" \
    SNAKE_DOCKER_VOLUMES=""

VOLUME /var/run/docker.sock /var/lib/snake-runner/secrets/

CMD ["/bin/snake-runner"]

# vim: ft=dockerfile
