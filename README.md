# Ermes wasm test

This repo contains a PoC that starts a WASM runtime and execute a WASM binary,
exposing two host functions that are used in the wasm runtime.
The host function performs GET and SET operations on a redis data store.

## Requirements

- [Golang](https://go.dev/dl/)
- [WasmEdge](https://wasmedge.org/docs/start/install)
- [Redis](https://redis.io/docs/latest/get-started/)

It is recommended to use the provided DevContainer, which already satisfies all the requirements.

## How to use the devcontainer

**Requirements:**

- Docker
- GoLand or ~~VSCode with the DevContainers extension~~.

### How to use with GOLand

1) Start Docker
2) Open GoLand
3) Open this project (or clone the repo if not already done)
4) A popup should ask to reopen the project in the container, click `reopen`

You can also start the container by navigating to the `devcontainer.json` file
and use the `Services` tab to manage existing containers.

**Known issue**: This DevContainer starts a Docker Compose, and it seems that GoLand is not able 
to stop it when the IDE is closed. You can stop it from the `Services` tab or from Docker Desktop

### How to use with VSCode

At the moment this DevContainer does not work with VSCode

## How to run

Run `go run main` from the DevContainers terminal or run the main function from GoLand.
