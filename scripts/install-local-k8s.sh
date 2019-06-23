#!/bin/sh -ex

sudo apt-get update
sudo apt-get dist-upgrade -qq
sudo apt-get autoremove -qq

sudo snap install microk8s --classic