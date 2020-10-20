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
RUN mv ./bin/harmony /root/harmony && chmod +x /root/harmony
RUN mv ./rosetta/infra/run.sh /root/run.sh && chmod +x /root/run.sh
RUN rm -rf $GOPATH

WORKDIR /root

ENTRYPOINT ["/bin/bash","/root/run.sh"]
