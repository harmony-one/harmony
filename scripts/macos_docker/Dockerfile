FROM ubuntu:22.04

ARG TARGETARCH
ARG GOLANG_VERSION="1.22.5"

SHELL ["/bin/bash", "-c"]

ENV GOPATH=/root/go
ENV GO111MODULE=on
ENV HMY_PATH=${GOPATH}/src/github.com/harmony-one
ENV OPENSSL_DIR=/usr/lib/ssl
ENV MCL_DIR=${HMY_PATH}/mcl
ENV BLS_DIR=${HMY_PATH}/bls
ENV CGO_CFLAGS="-I${BLS_DIR}/include -I${MCL_DIR}/include"
ENV CGO_LDFLAGS="-L${BLS_DIR}/lib"
ENV LD_LIBRARY_PATH=${BLS_DIR}/lib:${MCL_DIR}/lib
ENV GIMME_GO_VERSION=${GOLANG_VERSION}
ENV PATH="/root/bin:${PATH}"

RUN apt update && apt upgrade -y && \
	apt install libgmp-dev libssl-dev curl git \
	psmisc dnsutils jq make gcc g++ bash tig tree sudo vim \
	silversearcher-ag unzip emacs-nox nano bash-completion -y

RUN mkdir ~/bin && \
	curl -sL -o ~/bin/gimme \
	https://raw.githubusercontent.com/travis-ci/gimme/master/gimme && \
	chmod +x ~/bin/gimme

RUN eval "$(~/bin/gimme ${GIMME_GO_VERSION})"

#RUN git clone https://github.com/harmony-one/harmony.git ${HMY_PATH}/harmony

RUN git clone https://github.com/harmony-one/mcl.git ${HMY_PATH}/mcl && \
    echo "mcl repository cloned" && \
    ls -la ${HMY_PATH}/mcl  # List the contents of the mcl directory

RUN git clone https://github.com/harmony-one/bls.git ${HMY_PATH}/bls && \
    echo "bls repository cloned" && \
    ls -la ${HMY_PATH}/bls  # List the contents of the bls directory

RUN git clone https://github.com/harmony-one/go-sdk.git ${HMY_PATH}/go-sdk

RUN cd ${HMY_PATH}/bls && make -j8 BLS_SWAP_G=1

RUN touch /root/.bash_profile && \
	gimme ${GIMME_GO_VERSION} >> /root/.bash_profile && \
	echo "GIMME_GO_VERSION='${GIMME_GO_VERSION}'" >> /root/.bash_profile && \
	echo "GO111MODULE='on'" >> /root/.bash_profile && \
	echo ". ~/.bash_profile" >> /root/.profile && \
	echo ". ~/.bash_profile" >> /root/.bashrc

ENV PATH="/root/.gimme/versions/go${GIMME_GO_VERSION}.linux.${TARGETARCH:-amd64}/bin:${GOPATH}/bin:${PATH}"

RUN . ~/.bash_profile; \
	go install golang.org/x/tools/cmd/goimports; \
	go install golang.org/x/lint/golint ; \
	go install github.com/rogpeppe/godef ; \
	go install github.com/go-delve/delve/cmd/dlv; \
	go install github.com/golang/mock/mockgen; \
	go install github.com/stamblerre/gocode; \
	go install golang.org/x/tools/...; \
	go install honnef.co/go/tools/cmd/staticcheck/...

