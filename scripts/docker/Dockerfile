FROM alpine:3.16.0

ARG HARMONY_VERSION=v4.3.10
ARG HARMONY_USER=harmony
ARG HARMONY_USER_UID=1000
ARG HARMONY_USER_GID=1000

ENV HARMONY_HOME=/harmony
ENV HOME=${HARMONY_HOME}

RUN apk add --no-cache bash~=5.1.16-r2 bind-tools~=9.16.29-r0 tini~=0.19.0 curl==7.83.1-r1 sed~=4.8-r0 \
    && rm -rf /var/cache/apk/* \
    && addgroup -g ${HARMONY_USER_GID} ${HARMONY_USER} \
    && adduser -u ${HARMONY_USER_UID} -G ${HARMONY_USER} --shell /sbin/nologin --no-create-home -D ${HARMONY_USER} \
    && addgroup ${HARMONY_USER} tty \
    && sed -i -e "s/bin\/sh/bin\/bash/" /etc/passwd

RUN echo "[ ! -z \"\$TERM\" -a -r /etc/motd ] && cat /etc/motd" >> /etc/bash/bashrc \
    && echo "Version: ${HARMONY_VERSION}" > /etc/motd \
    && curl -LO https://harmony.one/hmycli \
    && chmod +x hmycli \
    && mv hmycli /usr/local/bin/hmy

WORKDIR ${HARMONY_HOME}

COPY bin/harmony ./scripts/docker/docker-entrypoint.sh ./

# Use testnet configuration as default
RUN chmod +x harmony docker-entrypoint.sh \
    && mv harmony /usr/local/bin/harmony \
    && harmony config dump --network testnet harmony.conf \
    && mkdir -p /data \
    && chown -R ${HARMONY_USER_UID}:${HARMONY_USER_GID} ${HARMONY_HOME} /data

VOLUME /data

USER ${HARMONY_USER_UID}:${HARMONY_USER_GID}

# NODE p2p (9000), pprof (6060), rpc (9500), ws (9800), rosetta (9700), DNS Sync (6000)
EXPOSE 9000/udp 9000/tcp 6060/tcp 9500/tcp 9800/tcp 9700/tcp 6000/tcp

ENTRYPOINT ["/sbin/tini", "--", "./docker-entrypoint.sh"]
CMD ["harmony", "-c", "harmony.conf"]
