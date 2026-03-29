#!/bin/bash

set -e

FOLDER=$1
REPO_NAME=$2
GITHUB_USERNAME="256dpi"
BRANCH_NAME="main"
BASE=$(pwd)

git config --global user.name "GitHub Actions"
git config --global user.email "actions@example.com"

echo "Syncing folder $FOLDER to $GITHUB_USERNAME/$REPO_NAME"

# Clone the target repository
git clone --depth 1 https://$API_TOKEN_GITHUB@github.com/$GITHUB_USERNAME/$REPO_NAME.git &> /dev/null
cd $REPO_NAME

# Remove all files in the repository except the .git folder
find . -maxdepth 1 ! -name '.git' -exec rm -rf {} \;

# Copy the contents of the source directory to the repository root
shopt -s dotglob
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
