FROM golang:1.24

ARG ENV=dev

RUN apt-get update && \
    apt-get install -y build-essential make git curl libgmp-dev libssl-dev gcc g++ && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /go/src/github.com/harmony-one

RUN git clone https://github.com/harmony-one/mcl.git && \
    git clone https://github.com/harmony-one/bls.git

RUN echo "Cloning branch: ${ENV}" && \
    git clone -b ${ENV} https://github.com/harmony-one/harmony.git && \
    cd harmony && \
    go mod tidy && \
    make deps

WORKDIR /go/src/github.com/harmony-one/harmony

CMD ["make", "go-test"]