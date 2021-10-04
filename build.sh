#!/bin/bash

user_in_group()
{
    groups $1 | grep -q "\b$2\b"
}

validate_user()
{
	if user_in_group $USER docker;
	then
	    echo "The user $USER is part of docker group; continue building....."
	else
		printf "The user $USER isn't part of the docker group, please add, verify the group membership is re-evaluated and re-run build.sh; exiting...\n"
		exit 1
	fi
}

#check if docker is installed; exit if otherwise
if [ -x "$(command -v docker)" ]; then
    if grep -q docker /etc/group;
    then
		validate_user 
    else
	printf "The docker group doesn't exist, please create docker group, add "$USER " to the docker group, verify the group membership is re-evaluated and re-run build.sh; exiting...\n"
	exit 1
    fi

else
    echo "please install docker on your system and re-run build.sh; exiting....\n"
	exit 1
fi

printf "\n ***** If you are behind proxies, please ensure proxy settings are configured at builder/Dockerfile *****\n "

#build docker image
builder/build_docker_image

#build binaries
builder/build

printf "\n ***** Build successful, the binaries are located in dist folder *****\n "
