#!/bin/bash -x
echo "Setup Golang" >> tmplog
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

# build executables
cd $GOPATH/src/harmony-benchmark
go build -o bin/soldier aws-experiment-launch/experiment/soldier/main.go
go build -o bin/benchmark benchmark.go
go build -o bin/txgen client/txgen/main.go