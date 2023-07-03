ARG GOPATH_DEFAULT=/root/go
ARG SRC_PATH=src/github.com/harmony-one

FROM golang:1.19-bullseye as builder

ARG GOPATH_DEFAULT
ARG SRC_PATH

ENV GOPATH=${GOPATH_DEFAULT}
ENV GO111MODULE=on
ENV HMY_PATH=${GOPATH}/${SRC_PATH}

ENV PACKAGES libgmp-dev libssl-dev curl git \
	psmisc dnsutils jq make gcc g++ bash tig tree

# hadolint ignore=DL3008
RUN apt-get update && apt-get install -y $PACKAGES --no-install-recommends

WORKDIR ${HMY_PATH}

RUN git clone https://github.com/harmony-one/harmony.git harmony \
  && git clone https://github.com/harmony-one/bls.git bls \
  && git clone https://github.com/harmony-one/mcl.git mcl \
  && git clone https://github.com/harmony-one/go-sdk.git go-sdk

WORKDIR ${HMY_PATH}/harmony
RUN make linux_static

# Pull harmony into a second stage deploy alpine container
FROM alpine:3.16.0

ARG GOPATH_DEFAULT
ARG SRC_PATH

ARG HARMONY_USER=harmony
ARG HARMONY_USER_UID=1000
ARG HARMONY_USER_GID=1000

ENV HARMONY_HOME=/harmony
ENV HOME=${HARMONY_HOME}

RUN apk add --no-cache bash bind-tools tini curl sed \
    && rm -rf /var/cache/apk/* \
    && addgroup -g ${HARMONY_USER_GID} ${HARMONY_USER} \
    && adduser -u ${HARMONY_USER_UID} -G ${HARMONY_USER} --shell /sbin/nologin --no-create-home -D ${HARMONY_USER} \
    && addgroup ${HARMONY_USER} tty \
    && sed -i -e "s/bin\/sh/bin\/bash/" /etc/passwd

RUN echo "[ ! -z \"\$TERM\" -a -r /etc/motd ] && cat /etc/motd" >> /etc/bash/bashrc

WORKDIR ${HARMONY_HOME}

COPY --from=builder ${GOPATH_DEFAULT}/${SRC_PATH}/harmony/bin/harmony /usr/local/bin/

RUN chmod +x  /usr/local/bin/harmony  \
    && mkdir -p /data \
    && chown -R ${HARMONY_USER_UID}:${HARMONY_USER_GID} ${HARMONY_HOME} /data

VOLUME /data

USER ${HARMONY_USER_UID}:${HARMONY_USER_GID}

# default base port, rpc port and rest port
EXPOSE 9000/tcp 9500/tcp 9800/tcp 6000/tcp

ENTRYPOINT ["tini", "--"]
CMD ["harmony", "-c", "harmony.conf"]
