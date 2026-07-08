FROM golang:1.24.2-bullseye

# hadolint ignore=DL3008
RUN apt-get update > /dev/null 2>&1 && \
    apt-get install -y protobuf-compiler --no-install-recommends && \
    protoc --version && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.30.0 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3 && \
    apt-get clean > /dev/null 2>&1 && \
    rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["protoc", "-I=/tmp", "--go_out=/tmp", "--go-grpc_out=/tmp"]
