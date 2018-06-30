#!/bin/bash -x
sudo yum update -y
sudo yum install -y golang

MyHOME=/home/ec2-user
echo "now setting up go-lang paths"
# GOROOT is the location where Go package is installed on your system
echo "export GOROOT=/usr/lib/golang" >> $MyHOME/.bash_profile

# GOPATH is the location of your work directory
echo "export GOPATH=$MyHOME/projects" >> $MyHOME/.bash_profile

# PATH in order to access go binary system wide
echo "export PATH=$PATH:$GOROOT/bin" >> $MyHOME/.bash_profile

export GOROOT=/usr/lib/golang
export GOPATH=$MyHOME/projects
export PATH=$PATH:$GOROOT/bin
source $MyHOME/.bash_profile
#sudo go get github.com/go-stack/stack