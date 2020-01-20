FROM ubuntu:18.04

SHELL ["/bin/bash", "-c"]

RUN apt update && apt upgrade -y

ENV GOPATH=/root/go
ENV GO111MODULE=on
ENV HMY_PATH=${GOPATH}/src/github.com/harmony-one
ENV OPENSSL_DIR=/usr/lib/ssl
ENV MCL_DIR=${HMY_PATH}/mcl
ENV BLS_DIR=${HMY_PATH}/bls
ENV CGO_CFLAGS="-I${BLS_DIR}/include -I${MCL_DIR}/include"
ENV CGO_LDFLAGS="-L${BLS_DIR}/lib"
ENV LD_LIBRARY_PATH=${BLS_DIR}/lib:${MCL_DIR}/lib
ENV GIMME_GO_VERSION="1.13.6"
ENV PATH="/root/bin:${PATH}"

RUN apt install libgmp-dev libssl-dev curl git \
jq make gcc g++ bash tig tree sudo vim \
silversearcher-ag unzip emacs-nox bash-completion -y

RUN mkdir ~/bin && curl -sL -o ~/bin/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme

RUN chmod +x ~/bin/gimme

RUN eval "$(~/bin/gimme ${GIMME_GO_VERSION})"

RUN mkdir /root/workspace

RUN git clone https://github.com/harmony-one/harmony.git ${HMY_PATH}/harmony

RUN git clone https://github.com/harmony-one/bls.git ${HMY_PATH}/bls

RUN git clone https://github.com/harmony-one/mcl.git ${HMY_PATH}/mcl

RUN git clone https://github.com/harmony-one/go-sdk.git ${HMY_PATH}/go-sdk

RUN cd ${HMY_PATH}/bls && make -j8 BLS_SWAP_G=1

RUN touch /root/.bash_profile

RUN gimme ${GIMME_GO_VERSION} >> /root/.bash_profile

RUN echo "GIMME_GO_VERSION='${GIMME_GO_VERSION}'" >> /root/.bash_profile

RUN echo "GO111MODULE='on'" >> /root/.bash_profile

RUN echo ". ~/.bash_profile" >> /root/.profile

RUN echo ". ~/.bash_profile" >> /root/.bashrc

ENV GOPATH='/root/go'

ENV PATH="/root/.gimme/versions/go${GIMME_GO_VERSION}.linux.amd64/bin:${GOPATH}/bin:${PATH}"

RUN eval "$(~/bin/gimme ${GIMME_GO_VERSION})" ; . ~/.bash_profile; \
go get -u golang.org/x/tools/cmd/goimports; \
go get -u golang.org/x/lint/golint ; \
go get -u github.com/rogpeppe/godef ; \
go get -u github.com/go-delve/delve/cmd/dlv; \
go get -u github.com/golang/mock/mockgen; \
go get -u github.com/stamblerre/gocode; \
go get -u golang.org/x/tools/...

WORKDIR ${HMY_PATH}/harmony

RUN eval "$(~/bin/gimme ${GIMME_GO_VERSION})" ; scripts/install_build_tools.sh

RUN eval "$(~/bin/gimme ${GIMME_GO_VERSION})" ; scripts/go_executable_build.sh

RUN cd ${HMY_PATH}/go-sdk && make -j8 && cp hmy /root/bin

RUN echo ". /etc/bash_completion" >> /root/.profile

RUN echo ". <(hmy completion)" >> /root/.profile

RUN echo ". /etc/bash_completion" >> /root/.bashrc

RUN echo ". <(hmy completion)" >> /root/.bashrc
