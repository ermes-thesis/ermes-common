# Ermes WASM pool

This library allows to create a pool of WASM runtimes which can execute WASM binaries.
The WASM binaries can export two type of functions:

- Authentication function: it receives a token (string) and returns a user identifier (string)
- Regular function: it receives the http request body (map string, string) and the Ermes Context,
  and returns an http code and a response body

To execute regular function, this library also expose some Host Functions that can be used in the WASM runtime.
The host function performs GET and SET operations on the redis data store and HTTP requests to the Central repository.

This repo also contains a test that starts a [Testcontainer](https://testcontainers.com/) with Redis and a WASM pool.
It then executes an auth function and a function that performs operation on some collections.

## Requirements

- [Golang](https://go.dev/dl/)
- [WasmEdge](https://wasmedge.org/docs/start/install)

It is recommended to use the provided Docker Compose or DevContainer, which already satisfies all the requirements.

## How to use

An example of usage can be found in the [Central Repo](https://github.com/ermes-thesis/ermes-central-repo).

## How to test

You can easily run test in a container by executing

```shell
docker compose -f .devcontainer/docker-compose.yml up
```

In alternative, you can use GoLand with a Docker Compose as a run target,
or the DevContainer, with a supported IDE (e.g. GoLand, VSCode).

Running locally should be possible, but requires additional configuration.

### How to use GoLand with the Docker Compose target (recommended)

This configuration allow to execute GoLand on the host machine, and uses the container only to build and run the
project.

1) Open the project with GoLand.
2) Start the docker-compose by pressing the play button in the `docker-compose.yml` file
3) GoLand will a run configuration `Test WASM Pool` that build and run/debug the project in a new container

The first time you need to set up the run target:

1) Click on `Test WASM Pool` and select `Edit configurations`
2) In the `Run on` menù, select `Create new target` > `Docker compose`
3) Select as configuration file the file `.devcontainer/docker-compose.yml`
4) Select the `go` service and insert the project name `ermes-wasm-pool`
5) Click `next` two times
6) In the `Additional Setting` section, in `Project sources directory` write `/workspaces/ermes-wasm-pool`
7) Click `build`
8) Click `Manage targets` and rename the new target as `Compose`

NB: When using the `Test WASM Pool` configuration, GoLand will leave some orphan container, just delete them manually.

### How to use the DevContainer

This configuration runs the IDE inside the DevContainer.
This requires more resources than the previous one, but it works out-of-the-box, without any additional configuration.

#### How to use with GOLand

1) Start Docker
2) Open GoLand
3) Open this project (or clone the repo if not already done)
4) A popup should ask to reopen the project in the container, click `reopen`

At this point to not use the `Test WASM Pool` configuration, but just run the test functions

You can also start the container by navigating to the `devcontainer.json` file
and use the `Services` tab to manage existing containers.

#### How to use with VSCode

1) Start Docker
2) Open the project in VSCode
3) A popup should ask to reopen the project in the container, click 'Reopen'
