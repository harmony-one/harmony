# no shebang; to be sourced from other scripts

unset -v gopath
gopath=$(go env GOPATH)
HMY_PATH="${gopath%%:*}/src/github.com/harmony-one"
: ${OPENSSL_DIR="/usr/local/opt/openssl"}
: ${MCL_DIR="${HMY_PATH}/mcl"}
: ${BLS_DIR="${HMY_PATH}/bls"}
export CGO_CFLAGS="-I${BLS_DIR}/include -I${MCL_DIR}/include"
export CGO_LDFLAGS="-L${BLS_DIR}/lib"
export LD_LIBRARY_PATH=${BLS_DIR}/lib:${MCL_DIR}/lib

OS=$(uname -s)
case $OS in
   Darwin)
      export CGO_CFLAGS="-I${BLS_DIR}/include -I${MCL_DIR}/include -I${OPENSSL_DIR}/include"
      export CGO_LDFLAGS="-L${BLS_DIR}/lib -L${OPENSSL_DIR}/lib"
      export LD_LIBRARY_PATH=${BLS_DIR}/lib:${MCL_DIR}/lib:${OPENSSL_DIR}/lib
      export DYLD_LIBRARY_PATH=$LD_LIBRARY_PATH
      ;;
esac
