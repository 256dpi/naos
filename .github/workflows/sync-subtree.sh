#!/bin/bash

set -euo pipefail

: "${API_TOKEN_GITHUB:?Missing API_TOKEN_GITHUB}"
: "${SYNC_PREFIX:?Missing SYNC_PREFIX}"
: "${SYNC_REMOTE_REPO:?Missing SYNC_REMOTE_REPO}"

SYNC_BRANCH="${SYNC_BRANCH:-main}"
SYNC_PUSH_TAGS="${SYNC_PUSH_TAGS:-1}"
REMOTE_NAME="sync-target"
TEMP_BRANCH="sync-$(echo "$SYNC_PREFIX" | tr '/' '-' | tr -cd '[:alnum:]-')-$$"

git config --global user.name "GitHub Actions"
git config --global user.email "actions@example.com"

echo "Syncing '$SYNC_PREFIX' history to $SYNC_REMOTE_REPO ($SYNC_BRANCH)"

if [ "$(git rev-parse --is-shallow-repository 2>/dev/null)" = "true" ]; then
  git fetch --prune --unshallow
fi
git fetch --prune --tags

git remote remove "$REMOTE_NAME" 2>/dev/null || true
git remote add "$REMOTE_NAME" "https://${API_TOKEN_GITHUB}@github.com/${SYNC_REMOTE_REPO}.git"

git branch -D "$TEMP_BRANCH" 2>/dev/null || true
if ! git subtree split --prefix="$SYNC_PREFIX" --branch "$TEMP_BRANCH" >/dev/null; then
  echo "Could not create subtree branch for '$SYNC_PREFIX'" >&2
  exit 1
fi

git push --force "$REMOTE_NAME" "$TEMP_BRANCH:$SYNC_BRANCH"

if [ "$SYNC_PUSH_TAGS" = "1" ]; then
  git for-each-ref --format='%(refname:strip=2)' refs/tags | while read -r tag; do
    if commit=$(git subtree split --prefix="$SYNC_PREFIX" "$tag" 2>/dev/null); then
      git push --force "$REMOTE_NAME" "$commit:refs/tags/$tag"
    fi
  done
fi

git remote remove "$REMOTE_NAME"
git branch -D "$TEMP_BRANCH" 2>/dev/null || true
