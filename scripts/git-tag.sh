#!/usr/bin/env sh

# This script is used to create a tag in the repository.

# Prompt to set the tag if not already set
if [ -z "$1" ]; then
  printf "Please enter the tag: "
  read -r GIT_TAG
else
  GIT_TAG=$1  
fi

# Create the tag and push it to the remote repository
git tag $GIT_TAG
git push origin --tags