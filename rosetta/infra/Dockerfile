# Build
FROM golang:1.16 AS build

RUN apt update -y && \
    apt install libgmp-dev libssl-dev git -y && \
    apt -y clean all

ENV GOPATH=/root/go
ENV GO111MODULE=on
ENV HMY_PATH=${GOPATH}/src/github.com/harmony-one
RUN mkdir -p $HMY_PATH

WORKDIR $HMY_PATH

RUN git clone https://github.com/harmony-one/harmony.git && \
    git clone https://github.com/harmony-one/bls.git && \
    git clone https://github.com/harmony-one/mcl.git

WORKDIR $HMY_PATH/harmony

RUN go mod download

RUN go mod tidy

RUN make linux_static && \
    cp ./bin/harmony /root/harmony && \
    cp ./rosetta/infra/run.sh /root/run.sh

RUN cp ./rosetta/infra/harmony-pstn.conf /root/harmony-pstn.conf && \
    cp ./rosetta/infra/harmony-mainnet.conf /root/harmony-mainnet.conf

# Execution
FROM ubuntu:latest

RUN apt update -y && \
    apt install libgmp-dev libssl-dev -y && \
    apt -y clean all

WORKDIR /root

COPY --from=build /root/harmony /root/harmony
COPY --from=build /root/run.sh /root/run.sh

EXPOSE 9500/tcp
EXPOSE 9700/tcp
EXPOSE 9800/tcp
ENTRYPOINT ["/bin/bash","/root/run.sh"]
