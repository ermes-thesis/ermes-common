FROM docker.io/tinygo/tinygo-dev

RUN go install golang.org/x/tools/gopls@latest && \
    go install golang.org/x/tools/cmd/goimports@latest && \
    go install github.com/sago35/tinygo-autocmpl@latest

RUN apt update && \
    apt install openocd -y
	
ENV GOPRIVATE=github.com/jackorse

RUN curl -sSf https://raw.githubusercontent.com/WasmEdge/WasmEdge/master/utils/install.sh | bash


RUN echo 'eval "$(tinygo-autocmpl --completion-script-bash)"' >> ~/.bashrc
