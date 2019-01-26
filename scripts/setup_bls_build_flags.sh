# no shebang; to be sourced from other scripts

HMY_PATH=$GOPATH/src/github.com/harmony-one
export CGO_CFLAGS="-I$HMY_PATH/bls/include -I$HMY_PATH/mcl/include"
export CGO_LDFLAGS="-L$HMY_PATH/bls/lib"
export LD_LIBRARY_PATH=$HMY_PATH/bls/lib:$HMY_PATH/mcl/lib

OS=$(uname -s)
case $OS in
   Darwin)
      export CGO_CFLAGS="-I$HMY_PATH/bls/include -I$HMY_PATH/mcl/include -I/usr/local/opt/openssl/include"
      export CGO_LDFLAGS="-L$HMY_PATH/bls/lib -L/usr/local/opt/openssl/lib"
      export LD_LIBRARY_PATH=$HMY_PATH/bls/lib:$HMY_PATH/mcl/lib:/usr/local/opt/openssl/lib
      export DYLD_LIBRARY_PATH=$LD_LIBRARY_PATH
      ;;
esac
