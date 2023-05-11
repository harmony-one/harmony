FROM golang:1.19-bullseye

RUN apt update
RUN apt install -y protobuf-compiler
RUN protoc --version
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.30.0
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3

ENTRYPOINT ["protoc", "-I=/tmp", "--go_out=/tmp", "--go-grpc_out=/tmp"]