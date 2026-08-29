FROM golang:1.24.2@sha256:30baaea08c5d1e858329c50f29fe381e9b7d7bced11a0f5f1f69a1504cdfbf5e

ARG MAIN_REPO_BRANCH=dev
ARG MAIN_REPO_ORG=harmony-one
ARG APT_SNAPSHOT=20260729T000000Z
ARG PYHMY_REF=5aeb8601fa174c734f9091619520cf3160b04a16
ARG MESH_CLI_VERSION=v0.10.4
ARG MESH_CLI_REF=8bdb815048e51fe0f6b821308070cc5c4b97073f
ARG HMY_VERSION=v2026.1.0
ARG HMY_AMD64_SHA256=3959f8474438c5139eef081e7185d09d69064bbb1be5b3835316dd727edda49f
ARG HMY_ARM64_SHA256=d2b4fd4f629fed65f1f4df485846b4f9ab9591cfcd4c20166e71cfb23a146c59

ENV BRANCH=${MAIN_REPO_BRANCH}
ENV MAIN_REPO=${MAIN_REPO_ORG}
ENV HMY_VERSION=${HMY_VERSION}
ENV DEBIAN_FRONTEND=noninteractive

SHELL ["/bin/bash", "-c"]
WORKDIR "$GOPATH/src/github.com/harmony-one"

RUN set -euo pipefail && \
    rm -f /etc/apt/sources.list.d/debian.sources && \
    printf '%s\n' \
      "deb [check-valid-until=no] https://snapshot.debian.org/archive/debian/${APT_SNAPSHOT} bookworm main" \
      "deb [check-valid-until=no] https://snapshot.debian.org/archive/debian/${APT_SNAPSHOT} bookworm-updates main" \
      "deb [check-valid-until=no] https://snapshot.debian.org/archive/debian-security/${APT_SNAPSHOT} bookworm-security main" \
      > /etc/apt/sources.list && \
    apt-get update > /dev/null && \
    apt-get install -y --no-install-recommends \
      jq=1.6-2.1+deb12u2 \
      python3-pip=23.0.1+dfsg-1 \
      unzip=6.0-28 \
      > /dev/null && \
    rm -rf /var/lib/apt/lists/*

RUN set -euo pipefail && \
    git init -q pyhmy && \
    git -C pyhmy remote add origin https://github.com/harmony-one/pyhmy.git && \
    git -C pyhmy fetch -q --depth=1 origin "$PYHMY_REF" && \
    git -C pyhmy checkout -q --detach FETCH_HEAD && \
    git config --global --add safe.directory "$GOPATH/src/github.com/harmony-one/harmony" && \
    git config --global --add safe.directory "$GOPATH/src/github.com/harmony-one/pyhmy"

WORKDIR "$GOPATH/src/github.com/coinbase/mesh-cli"
RUN set -euo pipefail && \
    case "$(dpkg --print-architecture)" in \
      amd64) HMY_ASSET=hmy-amd64; HMY_SHA256="$HMY_AMD64_SHA256" ;; \
      arm64) HMY_ASSET=hmy-arm64; HMY_SHA256="$HMY_ARM64_SHA256" ;; \
      *) echo "Unsupported architecture: $(dpkg --print-architecture)" >&2; exit 1 ;; \
    esac && \
    curl -fsSL -o /go/bin/hmy \
      "https://github.com/harmony-one/go-sdk/releases/download/${HMY_VERSION}/${HMY_ASSET}" && \
    printf '%s  %s\n' "$HMY_SHA256" /go/bin/hmy | sha256sum -c - && \
    chmod +x /go/bin/hmy && \
    git init -q . && \
    git remote add origin https://github.com/coinbase/mesh-cli.git && \
    git fetch -q --depth=1 origin "$MESH_CLI_REF" && \
    git checkout -q --detach FETCH_HEAD && \
    make install > /dev/null && \
    command -v rosetta-cli && \
    test "$(rosetta-cli version)" = "$MESH_CLI_VERSION"

WORKDIR "$GOPATH/src/github.com/harmony-one/harmony-test/localnet"
COPY scripts/ scripts/
COPY rpc_tests/ rpc_tests/
COPY configs/ configs/
COPY requirements.txt requirements.txt

RUN printf '%s\n' \
      'annotated-types==0.8.0' \
      'attrs==26.1.0' \
      'bitarray==3.9.2' \
      'certifi==2026.7.22' \
      'charset-normalizer==3.4.9' \
      'ckzg==2.1.8' \
      'cytoolz==1.0.1' \
      'eth-account==0.13.7' \
      'eth-hash==0.8.0' \
      'eth-keyfile==0.8.1' \
      'eth-keys==0.7.0' \
      'eth-rlp==2.2.0' \
      'eth-typing==6.0.0' \
      'eth-utils==5.3.0' \
      'eth_abi==6.0.0b1' \
      'execnet==2.1.2' \
      'flaky==3.7.0' \
      'hexbytes==1.3.0' \
      'idna==3.18' \
      'iniconfig==2.3.0' \
      'packaging==26.2' \
      'parsimonious==0.10.0' \
      'pexpect==4.9.0' \
      'pluggy==1.6.0' \
      'ptyprocess==0.7.0' \
      'py==1.11.0' \
      'pycryptodome==3.23.0' \
      'pydantic==2.13.4' \
      'pydantic_core==2.46.4' \
      'pytest==6.2.5' \
      'pytest-forked==1.6.0' \
      'pytest-ordering==0.6' \
      'pytest-xdist==1.33.0' \
      'regex==2026.7.19' \
      'requests==2.33.0' \
      'rlp==4.1.0' \
      'setuptools==80.9.0' \
      'six==1.17.0' \
      'toml==0.10.2' \
      'toolz==1.1.0' \
      'typing-inspection==0.4.2' \
      'typing_extensions==4.16.0' \
      'urllib3==2.7.0' \
      'wheel==0.45.1' \
      > /tmp/constraints.txt && \
    sed -i '/^pyhmy==/d' requirements.txt && \
    python3 -m pip install -c /tmp/constraints.txt \
      setuptools==80.9.0 wheel==0.45.1 \
      --break-system-packages --no-cache-dir > /dev/null && \
    python3 -m pip install -c /tmp/constraints.txt -r requirements.txt \
      --break-system-packages --no-cache-dir > /dev/null && \
    rm requirements.txt && \
    chmod +x scripts/run.sh

WORKDIR "$GOPATH/src/github.com/harmony-one/pyhmy"
RUN python3 -m pip install -c /tmp/constraints.txt \
      --no-build-isolation --break-system-packages --no-cache-dir . > /dev/null

WORKDIR "$GOPATH/src/github.com/harmony-one/harmony"
ENTRYPOINT ["/go/src/github.com/harmony-one/harmony-test/localnet/scripts/run.sh"]
