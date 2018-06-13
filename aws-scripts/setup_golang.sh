#!/bin/bash -x
sudo yum update -y
sudo yum install -y golang

# GOROOT is the location where Go package is installed on your system
echo "export GOROOT=/usr/lib/golang" >> $HOME/.bash_profile

# GOPATH is the location of your work directory
echo "export GOPATH=/home/ec2-user/projects" >> $HOME/.bash_profile

# PATH in order to access go binary system wide
echo "export PATH=$PATH:$GOROOT/bin" >> $HOME/.bash_profile