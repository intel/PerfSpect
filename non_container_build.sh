#!/bin/bash

### non container build script
### Assumes the following are installed
	#python3.6 or above
	#golang1.13 or above
rm -rf build
rm -rf dist
pip3 install -r requirements.txt
pip3 install pyinstaller==4.5.1
pip3 install flake8
pip3 install black
pip3 install bandit
go get github.com/markbates/pkger/cmd/pkger
make dist -f Makefile_non_container
rm -rf __pycache*
rm -f *.spec

