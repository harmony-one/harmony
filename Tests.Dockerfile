FROM golang:1.22

WORKDIR $GOPATH/src/github.com/harmony-one

SHELL ["/bin/bash", "-c"]

# These are split into multiple lines to allow debugging the error message that I cannot reproduce locally
# The command `xxx` returned a non-zero code: 100
RUN apt clean > /dev/null 2>1
RUN apt update > /dev/null 2>1
RUN apt upgrade -y > /dev/null 2>1
RUN apt update -y > /dev/null 2>1
RUN apt install -y unzip libgmp-dev libssl-dev curl git jq make gcc g++ bash sudo python3 python3-pip

#RUN #git clone https://github.com/harmony-one/harmony.git > /dev/null 2>1 \
RUN git clone https://github.com/harmony-one/bls.git > /dev/null 2>1
RUN git clone https://github.com/harmony-one/mcl.git > /dev/null 2>1

# Fix complaints about Docker / root / user ownership for Golang "VCS stamping"
# https://github.com/golang/go/blob/3900ba4baf0e3b309a55b5ac4dd25f709df09772/src/cmd/go/internal/vcs/vcs.go
RUN git config --global --add safe.directory $GOPATH/src/github.com/harmony-one/harmony > /dev/null 2>1 \
    && git config --global --add safe.directory $GOPATH/src/github.com/harmony-one/bls > /dev/null 2>1 \
    && git config --global --add safe.directory $GOPATH/src/github.com/harmony-one/mcl > /dev/null 2>1

# Install testing tools
RUN curl -L -o /go/bin/hmy https://harmony.one/hmycli > /dev/null 2>1 && chmod +x /go/bin/hmy > /dev/null 2>1

RUN git clone https://github.com/coinbase/rosetta-cli.git > /dev/null 2>1
RUN cd rosetta-cli && make install > /dev/null 2>1

# Build to fetch all dependencies for faster test builds
WORKDIR $GOPATH/src/github.com/harmony-one/harmony
COPY . .
# Выполняем команду pwd для вывода текущего каталога
RUN echo "Current directory:" && pwd

# Проверяем содержимое каталога
RUN echo "Listing directory contents:" && ls -la

RUN go mod tidy
#RUN go get github.com/pborman/uuid > /dev/null 2>1
#RUN go get github.com/rjeczalik/notify > /dev/null 2>1
#RUN go get github.com/cespare/cp > /dev/null 2>1
#RUN go get github.com/libp2p/go-libp2p-crypto > /dev/null 2>1
#RUN go get github.com/kr/pretty > /dev/null 2>1
#RUN go get github.com/kr/text > /dev/null 2>1
#RUN go get gopkg.in/check.v1 > /dev/null 2>1
RUN bash scripts/install_build_tools.sh > /dev/null 2>1
RUN make
#RUN rm -rf harmony


WORKDIR $GOPATH/src/github.com/coinbase


WORKDIR $GOPATH/src/github.com/harmony-one/harmony/tests

#COPY scripts scripts
#COPY rpc_tests rpc_tests
#COPY configs configs
#COPY requirements.txt requirements.txt

WORKDIR $GOPATH/src/github.com/harmony-one/harmony

# Since we are running as root in Docker, `--break-system-packages` is required
RUN python3 -m pip install -r tests/requirements.txt --break-system-packages
RUN chmod +x tests/scripts/run.sh


ENTRYPOINT ["/go/src/github.com/harmony-one/harmony/tests/scripts/run.sh"]
