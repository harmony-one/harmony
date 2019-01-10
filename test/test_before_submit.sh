#!/bin/bash

DIRROOT=$(dirname $0)/..
OS=$(uname -s)

go test ./...

pushd $DIRROOT
./scripts/.travis_checker.sh

case $OS in
   Darwin)
      ./go_executable_build.sh -o darwin
      ;;
   Linux)
      ./go_executable_build.sh
      ;;
esac

popd
