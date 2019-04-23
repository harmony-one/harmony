FROM alpine

RUN apk add --no-cache bash libstdc++ gmp-dev libc6-compat && ln -s libcrypto.so.1.1 /lib/libcrypto.so.10

# default base port, rpc port and rest port
EXPOSE 9000/tcp 14555/tcp 6000/tcp

VOLUME ["/harmony/db", "/harmony/log"]

# Default BN for cello
ENV BN_MA "/ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv,/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9"
ENV NODE_PORT "9000"
ENV NODE_ACCOUNT_ID ""

ENTRYPOINT ["/bin/run"]
WORKDIR /harmony

COPY run /bin/run
COPY libbls384.so libmcl.so /lib/
COPY harmony /bin/
