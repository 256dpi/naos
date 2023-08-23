#!/bin/bash

set -e

FOLDER="swift/NAOSKit"
GITHUB_USERNAME="256dpi"
REPO_NAME="NAOSKit"
BRANCH_NAME="main"
BASE=$(pwd)

git config --global user.name "GitHub Actions"
git config --global user.email "actions@example.com"

echo "Cloning folder $FOLDER and pushing to $GITHUB_USERNAME"

cd $FOLDER
FOLDER_NAME=${PWD##*/}
cd $BASE

echo "  Name: $REPO_NAME"
CLONE_DIR="__${REPO_NAME}__clone__"
echo "  Clone dir: $CLONE_DIR"

# clone, delete files in the clone, and copy (new) files over
# this handles file deletions, additions, and changes seamlessly
git clone --depth 1 https://$API_TOKEN_GITHUB@github.com/$GITHUB_USERNAME/$REPO_NAME.git $CLONE_DIR &> /dev/null
cd $CLONE_DIR
[ -d $FOLDER_NAME ] && find ./$FOLDER_NAME | grep -v ".git" | grep -v "^\.*$" | xargs rm -rf

# delete all files only in that folder if folder exists
mkdir -p ./$FOLDER_NAME
cp -r $BASE/$FOLDER/. ./$FOLDER_NAME
echo "  Copied files to $FOLDER_NAME"

# Commit if there is anything to
if [ -n "$(git status --porcelain)" ]; then
  echo  "  Committing $REPO_NAME to $GITHUB_REPOSITORY"
  git add .
  git commit --message "Update $REPO_NAME from $GITHUB_REPOSITORY"
  git push origin $BRANCH_NAME
  echo  "  Completed $REPO_NAME"
else
  echo "  No changes, skipping $BASE/$FOLDER/"
fi

cd ..
rm -r $CLONE_DIR
cd $BASE
