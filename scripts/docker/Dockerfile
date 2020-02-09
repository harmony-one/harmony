FROM alpine

RUN apk add --no-cache bash libstdc++ gmp-dev libc6-compat bind-tools jq && ln -s libcrypto.so.1.1 /lib/libcrypto.so.10

# default base port, rpc port and rest port
EXPOSE 9000/tcp 9500/tcp 9800/tcp 6000/tcp

VOLUME ["/harmony/harmony_db_0","/harmony/harmony_db_1","/harmony/harmony_db_2","/harmony/harmony_db_3","/harmony/log","/harmony/.hmy"]

# Default BN for S3/mainnet
ENV NODE_BN_MNET "/ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv,/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9,/ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX,/ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj"

# Long Running Test Net
ENV NODE_BN_LRTN "/ip4/54.213.43.194/tcp/9868/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9,/ip4/100.26.90.187/tcp/9868/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv,/ip4/13.113.101.219/tcp/12018/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX"

# Open Staking Test Net
ENV NODE_BN_OSTN "/ip4/54.86.126.90/tcp/9880/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv,/ip4/52.40.84.2/tcp/9880/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29"

ENV NODE_PORT "9000"
ENV NODE_BLSKEY ""
ENV NODE_BLSPASS ""
ENV NODE_DNS_ZONE "t.hmny.io"
ENV NODE_RPC "true"
ENV NODE_BLACKLIST ""
ENV NODE_NETWORK_TYPE "mainnet"

ENTRYPOINT ["/bin/run"]
WORKDIR /harmony

COPY run /bin/run
COPY harmony /bin/
