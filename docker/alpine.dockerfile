FROM alpine:3

COPY /snake-runner.docker /bin/snake-runner

RUN  mkdir /etc/snake-runner/
COPY /conf/snake-runner.conf /etc/snake-runner/snake-runner.conf

ENV SNAKE_MASTER_ADDRESS         ""
ENV SNAKE_LOG_DEBUG              ""
ENV SNAKE_LOG_TRACE              ""
ENV SNAKE_NAME                   ""
ENV SNAKE_TOKEN                  ""
ENV SNAKE_TOKEN_PATH             ""
ENV SNAKE_VIRTUALIZATION         ""
ENV SNAKE_MAX_PARALLEL_PIPELINES ""
ENV SNAKE_SSH_KEY_PATH           ""

VOLUME /var/run/docker.sock
VOLUME /var/lib/snake-runner/secrets/

CMD ["/bin/snake-runner"]

# vim: ft=dockerfile
