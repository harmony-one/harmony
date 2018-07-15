#!/bin/bash -x
echo "Setup Golang" >> tmplog
#sudo yum update -y

sudo yum install -y golang
sudo yum install -y git
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

cd $GOPATH/src/harmony-benchmark
touch 'yum_not_updated.txt'
# go get dependencies
go get ./...
curl --silent http://169.254.169.254/latest/meta-data/public-ipv4 >> bin/myip.txt
# build executables
go build -o bin/soldier aws-experiment-launch/experiment/soldier/main.go
go build -o bin/commander aws-experiment-launch/experiment/commander/main.go
go build -o bin/benchmark benchmark.go
go build -o bin/txgen client/txgen/main.go

# Setup ulimit
echo "* soft     nproc          65535" | sudo tee -a /etc/security/limits.conf
echo "* hard     nproc          65535" | sudo tee -a /etc/security/limits.conf
echo "* soft     nofile         65535" | sudo tee -a /etc/security/limits.conf
echo "* hard     nofile         65535" | sudo tee -a /etc/security/limits.conf
echo "root soft     nproc          65535" | sudo tee -a /etc/security/limits.conf
echo "root hard     nproc          65535" | sudo tee -a /etc/security/limits.conf
echo "root soft     nofile         65535" | sudo tee -a /etc/security/limits.conf
echo "root hard     nofile         65535" | sudo tee -a /etc/security/limits.conf
echo "session required pam_limits.so" | sudo tee -a /etc/pam.d/common-session
