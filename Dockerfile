FROM ubuntu:18.04

SHELL ["/bin/bash", "-c"]

RUN apt update && apt upgrade -y

RUN apt install libgmp-dev libssl-dev curl git \
jq make gcc g++ bash tig tree sudo \
silversearcher-ag unzip emacs-nox -y

RUN mkdir ~/bin && curl -sL -o ~/bin/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme

RUN chmod +x ~/bin/gimme

RUN eval "$(~/bin/gimme 1.12.9)"

RUN mkdir /root/workspace

RUN git clone https://github.com/harmony-one/harmony.git /root/workspace/harmony

RUN git clone https://github.com/harmony-one/bls.git /root/workspace/bls

RUN git clone https://github.com/harmony-one/mcl.git /root/workspace/mcl

RUN git clone https://github.com/harmony-one/go-sdk.git /root/workspace/go-sdk

RUN cd /root/workspace/bls && make -j8 BLS_SWAP_G=1

ENV PATH="/root/bin:${PATH}"

ENV GIMME_GO_VERSION="1.12.9"

RUN touch /root/.bash_profile

RUN gimme 1.12.9 >> /root/.bash_profile

RUN echo "GIMME_GO_VERSION='1.12.9'" >> /root/.bash_profile

RUN echo "GO111MODULE='on'" >> /root/.bash_profile

RUN echo ". ~/.bash_profile" >> /root/.profile

RUN echo ". ~/.bash_profile" >> /root/.bashrc

ENV GOPATH='/root/go'

ENV PATH="/root/.gimme/versions/go1.12.9.linux.amd64/bin:/root/go/bin:${PATH}"

RUN eval "$(~/bin/gimme 1.12.9)" ; . ~/.bash_profile; \
go get -u golang.org/x/tools/cmd/goimports; \
go get -u golang.org/x/lint/golint ; \
go get -u github.com/rogpeppe/godef ; \
go get -u github.com/go-delve/delve/cmd/dlv; \
go get -u github.com/golang/mock/mockgen; \
go get -u github.com/stamblerre/gocode; \
go get -u golang.org/x/tools/...

ENV GO111MODULE="on"

WORKDIR /root/workspace/harmony

RUN git remote add fork https://github.com/fxfactorial/harmony.git && git fetch fork && git fetch origin

RUN echo $'\n\
[user] \n\
# Please adapt and uncomment the following lines: \n\
	name = Edgar Aroutiounian \n\
	email = edgar.factorial@gmail.com \n\
[push] \n\
	default = simple \n\
[alias] \n\
	cp = cherry-pick \n\
	s = status \n\
	b = branch \n\
	d = diff \n\
	co = checkout \n\
	r = reset --hard \n\
[filter "lfs"] \n\
	clean = git-lfs clean -- %f \n\
	smudge = git-lfs smudge -- %f \n\
	required = true \n\
	process = git-lfs filter-process \n\
[color] \n\
	ui = auto \n\
[tig] \n\
	diff-view = line-number:yes,interval=5 text \n\
[submodule] \n\
	recurse = true' > /root/.gitconfig 

RUN eval "$(~/bin/gimme 1.12.9)" ; scripts/install_build_tools.sh

RUN eval "$(~/bin/gimme 1.12.9)" ; scripts/go_executable_build.sh
