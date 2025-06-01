# Ermes wasm test

This repo contains a PoC that starts a WASM runtime and execute a WASM binary,
exposing two host functions that are used in the wasm runtime.
The host function performs GET and SET operations on a redis data store.

## Requirements

- [Golang](https://go.dev/dl/)
- [WasmEdge](https://wasmedge.org/docs/start/install)
- [Redis](https://redis.io/docs/latest/get-started/)

It is recommended to use the provided Docker Compose or DevContainer, which already satisfies all the requirements.

## How to run

To run this project you should either use GoLand with a Docker Compose as a run target,
or the DevContainer, with a supported IDE (e.g. GoLand, VSCode).

Running locally should be possible, but requires additional configuration.

### How to use GoLand with the Docker Compose target (recommended)

This configuration allow to execute GoLand on the host machine, and uses the container only to build and run the
project.

1) Open the project with GoLand.
2) Start the docker-compose by pressing the play button in the `docker-compose.yml` file
3) GoLand will have 2 running configuration
    1) `Run in new container`: this will build and run the project in a new container, allowing also to debug
    2) `Run in container`: this will build and run the project in the running container. This is faster, but does not
       allow to debug

The first time you run the `Run in new container` you need to setup the run target:

1) Click on `Run in new container` and select `Edit configurations`
2) In the `Run on` menù, select `Create new target` > `Docker compose`
3) Select as configuration file the file `.devcontainer/docker-compose.yml`
4) Select the `go` service and insert the project name `ermes-wasm-test`
5) Click `next` two times
6) In the `Additional Setting` section, in `Project sources directory` write `/workspaces/ermes-wasm-test`
7) Click `build`
8) Click `Manage targets` and rename the new target as `Compose`

NB: When using the `Run in new container` option, GoLand will leave some orphan container, just delete them manually.

### How to use the DevContainer

This configuration runs the IDE inside the DevContainer.
This requires more resources than the previous one, but it works out-of-the-box, without any additional configuration.

#### How to use with GOLand

1) Start Docker
2) Open GoLand
3) Open this project (or clone the repo if not already done)
4) A popup should ask to reopen the project in the container, click `reopen`

You can also start the container by navigating to the `devcontainer.json` file
and use the `Services` tab to manage existing containers.

**Known issue**: This DevContainer starts a Docker Compose, and it seems that GoLand is not able
to stop it when the IDE is closed. You can stop it from the `Services` tab or from Docker Desktop

#### How to use with VSCode

1) Start Docker
2) Open the project in VSCode
3) A popup should ask to reopen the project in the container, click 'Reopen'
