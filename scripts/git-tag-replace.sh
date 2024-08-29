#!/usr/bin/env sh

# This script is used to replace an existing tag in the repository.

# Prompt to set the tag if not already set
if [ -z "$1" ]; then
  printf "Please enter the tag: "
  read -r GIT_TAG
else
  GIT_TAG=$1  
fi

# Delete the tag from the remote repository and locally
git push origin :refs/tags/$GIT_TAG
git tag --delete $GIT_TAG

# Create a new tag and push it to the remote repository
git tag $GIT_TAG
git push origin --tags