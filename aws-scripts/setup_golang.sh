#!/bin/bash -x
sudo yum update -y
sudo yum install -y golang


echo "now setting up go-lang paths"
# GOROOT is the location where Go package is installed on your system
echo "export GOROOT=/usr/lib/golang" >> $HOME/.bash_profile

# GOPATH is the location of your work directory
echo "export GOPATH=$HOME/projects" >> $HOME/.bash_profile

# PATH in order to access go binary system wide
echo "export PATH=$PATH:$GOROOT/bin" >> $HOME/.bash_profile

export GOROOT=/usr/lib/golang
export GOPATH=$HOME/projects
export PATH=$PATH:$GOROOT/bin
source $HOME/.bash_profile
sudo go get github.com/go-stack/stack