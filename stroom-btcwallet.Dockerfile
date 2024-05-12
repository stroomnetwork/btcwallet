FROM ubuntu:22.04

# TODO this updates certificates that enable to connect to https
RUN apt-get update && \
    apt install -y software-properties-common

RUN mkdir /stroom

WORKDIR /stroom
COPY ./btcwallet /stroom
# TODO Run container as non-root user with absolute minimum of permissions. See: https://www.redhat.com/en/blog/understanding-root-inside-and-outside-container
ENTRYPOINT [ "/stroom/btcwallet" ]
