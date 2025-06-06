FROM golang:1.24

ENV GOPRIVATE=github.com/ermes-thesis
ENV TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal

RUN curl -sSf https://raw.githubusercontent.com/WasmEdge/WasmEdge/master/utils/install.sh | bash -s -- -p /usr/local
