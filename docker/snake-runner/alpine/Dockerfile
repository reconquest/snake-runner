FROM alpine:3

COPY /snake-runner.docker /bin/snake-runner

RUN  mkdir /etc/snake-runner/
COPY /conf/snake-runner.conf /etc/snake-runner/snake-runner.conf

VOLUME /var/run/docker.sock /var/lib/snake-runner/secrets/

CMD ["/bin/snake-runner"]

# vim: ft=dockerfile
