FROM ubuntu:18.04

ARG TARGETARCH
ARG GOLANG_VERSION="1.19"

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

RUN git clone https://github.com/harmony-one/harmony.git ${HMY_PATH}/harmony

RUN git clone https://github.com/harmony-one/bls.git ${HMY_PATH}/bls

RUN git clone https://github.com/harmony-one/mcl.git ${HMY_PATH}/mcl

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

WORKDIR ${HMY_PATH}/harmony

RUN scripts/install_build_tools.sh

RUN go mod tidy

RUN scripts/go_executable_build.sh -S

RUN cd ${HMY_PATH}/go-sdk && make -j8 && cp hmy /root/bin

ARG K1=one1tq4hy947c9gr8qzv06yxz4aeyhc9vn78al4rmu
ARG K2=one1y5gmmzumajkm5mx3g2qsxtza2d3haq0zxyg47r
ARG K3=one1qrqcfek6sc29sachs3glhs4zny72mlad76lqcp

ARG KS1=8d222cffa99eb1fb86c581d9dfe7d60dd40ec62aa29056b7ff48028385270541
ARG KS2=da1800da5dedf02717696675c7a7e58383aff90b1014dfa1ab5b7bd1ce3ef535
ARG KS3=f4267bb5a2f0e65b8f5792bb6992597fac2b35ebfac9885ce0f4152c451ca31a

RUN hmy keys import-private-key ${KS1} && \
	hmy keys import-private-key ${KS2} && \
	hmy keys import-private-key ${KS3} && \
	hmy keys generate-bls-key > keys.json

RUN jq  '.["encrypted-private-key-path"]' -r keys.json > /root/keypath && cp keys.json /root && \
	echo "export BLS_KEY_PATH=$(cat /root/keypath)" >> /root/.bashrc && \
	echo "export BLS_KEY=$(jq '.["public-key"]' -r keys.json)" >> /root/.bashrc && \
	echo "printf '${K1}, ${K2}, ${K3} are imported accounts in hmy for local dev\n\n'" >> /root/.bashrc && \
	echo "printf 'test with: hmy blockchain validator information ${K1}\n\n'" >> /root/.bashrc && \
	echo "echo "$(jq '.["public-key"]' -r keys.json)" is an extern bls key" >> /root/.bashrc && \
	echo ". /etc/bash_completion" >> /root/.bashrc && \
	echo ". <(hmy completion)" >> /root/.bashrc
