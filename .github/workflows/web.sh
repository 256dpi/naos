#!/bin/bash

set -e

FOLDER="web"
GITHUB_USERNAME="256dpi"
REPO_NAME="NAOS.js"
BRANCH_NAME="main"
BASE=$(pwd)

git config --global user.name "GitHub Actions"
git config --global user.email "actions@example.com"

echo "Cloning folder $FOLDER and pushing to $GITHUB_USERNAME"

# Clone the target repository
git clone --depth 1 https://$API_TOKEN_GITHUB@github.com/$GITHUB_USERNAME/$REPO_NAME.git &> /dev/null
cd $REPO_NAME

# Remove all files in the repository except the .git folder
find . -maxdepth 1 ! -name '.git' -exec rm -rf {} \;

# Copy the contents of the source directory to the repository root
shopt -s dotglob # also copy hidden files
cp -r $BASE/$FOLDER/* .

# Commit and push changes if there are any
if [ -n "$(git status --porcelain)" ]; then
  echo "Committing and pushing changes"
  git add .
  git commit --message "Update from $GITHUB_REPOSITORY"
  git push origin $BRANCH_NAME
  echo "Completed"
else
  echo "No changes to commit"
fi

cd ..
rm -rf $REPO_NAME
cd $BASE
