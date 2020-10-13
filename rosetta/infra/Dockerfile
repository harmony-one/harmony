FROM golang:1.14

RUN apt update -y && apt upgrade -y
RUN apt install libgmp-dev libssl-dev curl git -y

ENV GOPATH=/root/go
ENV GO111MODULE=on
ENV HMY_PATH=${GOPATH}/src/github.com/harmony-one
RUN mkdir -p $HMY_PATH

WORKDIR $HMY_PATH

RUN git clone https://github.com/harmony-one/harmony.git
RUN git clone https://github.com/harmony-one/bls.git
RUN git clone https://github.com/harmony-one/mcl.git

WORKDIR $HMY_PATH/harmony

RUN make linux_static
RUN cp ./bin/harmony /root && chmod +x /root/harmony

WORKDIR $GOPATH

RUN rm -rf *

WORKDIR /root

COPY run.sh run.sh
ENTRYPOINT ["/bin/bash","/root/run.sh"]
