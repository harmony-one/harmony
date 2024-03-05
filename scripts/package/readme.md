# Introduction
This document introduces the Harmony's package release using standard packaging system, RPM and Deb packages.

Standard packaging system has many benefits, like extensive tooling, documentation, portability, and complete design to handle different situation.

# Package Content
The RPM/Deb packages will install the following files/binary in your system.
* /usr/sbin/harmony
* /usr/sbin/harmony-setup.sh
* /usr/sbin/harmony-rclone.sh
* /etc/harmony/harmony.conf
* /etc/harmony/rclone.conf
* /etc/systemd/system/harmony.service
* /etc/sysctl.d/99-harmony.conf

The package will create `harmony` group and `harmony` user on your system.
The harmony process will be run as `harmony` user.
The default blockchain DBs are stored in `/home/harmony/harmony_db_?` directory.
The configuration of harmony process is in `/etc/harmony/harmony.conf`.

# Package Manager
Please take some time to learn about the package managers used on Fedora/Debian based distributions.
There are many other package managers can be used to manage rpm/deb packages like [Apt]<https://en.wikipedia.org/wiki/APT_(software)>,
or [Yum]<https://www.redhat.com/sysadmin/how-manage-packages>

# Setup customized repo
You just need to do the setup of harmony repo once on a new host.
**TODO**: the repo in this document are for development/testing purpose only.

Official production repo will be different.

## RPM Package
RPM is for Redhat/Fedora based Linux distributions, such as Amazon Linux and CentOS.

```bash
# do the following once to add the harmony development repo
curl -LsSf http://haochen-harmony-pub.s3.amazonaws.com/pub/yum/harmony-dev.repo | sudo tee -a /etc/yum.repos.d/harmony-dev.repo
sudo rpm --import https://raw.githubusercontent.com/harmony-one/harmony-open/master/harmony-release/harmony-pub.key
```

## Deb Package
Deb is supported on Debian based Linux distributions, such as Ubuntu, MX Linux.

```bash
# do the following once to add the harmony development repo
curl -LsSf https://raw.githubusercontent.com/harmony-one/harmony-open/master/harmony-release/harmony-pub.key | sudo apt-key add
echo "deb http://haochen-harmony-pub.s3.amazonaws.com/pub/repo bionic main" | sudo tee -a /etc/apt/sources.list

```

# Test cases
## installation
```
# debian/ubuntu
sudo apt-get update
sudo apt-get install harmony

# fedora/amazon linux
sudo yum install harmony
```
## configure/start
```
# dpkg-reconfigure harmony (TODO)
sudo systemctl start harmony
```

## uninstall
```
# debian/ubuntu
sudo apt-get remove harmony

# fedora/amazon linux
sudo yum remove harmony
```

## upgrade
```bash
# debian/ubuntu
sudo apt-get update
sudo apt-get upgrade

# fedora/amazon linux
sudo yum update --refresh
```

## reinstall
```bash
remove and install
```

# Rclone
## install latest rclone
```bash
# debian/ubuntu
curl -LO https://downloads.rclone.org/v1.52.3/rclone-v1.52.3-linux-amd64.deb
sudo dpkg -i rclone-v1.52.3-linux-amd64.deb

# fedora/amazon linux
curl -LO https://downloads.rclone.org/v1.52.3/rclone-v1.52.3-linux-amd64.rpm
sudo rpm -ivh rclone-v1.52.3-linux-amd64.rpm
```

## do rclone
```bash
# validator runs on shard1
sudo -u harmony harmony-rclone.sh /home/harmony 0
sudo -u harmony harmony-rclone.sh /home/harmony 1

# explorer node
sudo -u harmony harmony-rclone.sh -a /home/harmony 0
```

# Setup explorer (non-validating) node
To setup an explorer node (non-validating) node, please run the `harmony-setup.sh` at first.

```bash
sudo /usr/sbin/harmony-setup.sh -t explorer -s 0
```
to setup the node as an explorer node w/o blskey setup.

# Setup new validator
Please copy your blskey to `/home/harmony/.hmy/blskeys` directory, and start the node.
The default configuration is for validators on mainnet. No need to run `harmony-setup.sh` script.

# Start/stop node
* `systemctl start harmony` to start node
* `systemctl stop harmony` to stop node
* `systemctl status harmony` to check status of node

# Change node configuration
The node configuration file is in `/etc/harmony/harmony.conf`.  Please edit the file as you need.
```bash
sudo vim /etc/harmony/harmony.conf
```

# Support
Please open new github issues in https://github.com/harmony-one/harmony/issues.
