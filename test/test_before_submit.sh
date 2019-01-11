#!/bin/bash

DIRROOT=$(dirname $0)/..
OS=$(uname -s)

pushd $DIRROOT
./scripts/travis_checker.sh

case $OS in
   Darwin)
      ./scripts/go_executable_build.sh -o darwin
      ;;
   Linux)
      ./scripts/go_executable_build.sh
      ;;
esac

popd
