# Ermes wasm test

This repo contains :
- a devcontainer with all the requirements to run a WasmEdge runtime
- an example of GO code that start a wasm runtime and execute a function, passing parameters and obtaining a result


## How to use the devcontainer
**Requirements:**
- Docker
- GoLand or VSCode with the devcontainers extension

### How to use with GOLand
1) Start Docker
2) Open GoLand
3) At this point
    1) if you are in the splash screen, navigate to the DevContainer page
    2) if you already have an opened project click on file -> remote development
4) Click on 'new devcontainer'
5) Select 'from VSC project' and past the link of this repo
6) Click build (the first time it will take a while and may fail, just click on restart)

When you have opened a local project, you can also manage devcontainers from the 'Services' tab

### How to use with VSCode
1) start Docker
2) Open the project in VSCode
3) A popup should ask to reopen the project in the container, click 'Reopen'
   ... Not tested yet, but should work ...

## How to test
Run `go run main` from the devcontainer terminal or use the provided GoLand run configuration.
This will spawn a WasmEdge runtime, passing some random context, executes the `function.wasm` binary and obtain the result
