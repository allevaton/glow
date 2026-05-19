#!/bin/sh

# Syncs this fork with upstream:
#   1. Fetch upstream
#   2. Fast-forward main to upstream/main (bails if main has diverged)
#   3. Push main to origin
#   4. Rebase the current branch onto main
#
# Run from any branch you want rebased (typically `local`).
# This script is gitignored — it's a personal workflow helper.

set -e

current_branch=$(git rev-parse --abbrev-ref HEAD)

if [ "$current_branch" = "main" ]; then
    echo "Refusing to run from main — check out your work branch first."
    exit 1
fi

if ! git diff-index --quiet HEAD --; then
    echo "Working tree is dirty. Commit or stash first."
    exit 1
fi

echo "==> Fetching upstream"
git fetch upstream

echo "==> Fast-forwarding main to upstream/main"
git checkout main
git merge --ff-only upstream/main

echo "==> Pushing main to origin"
git push origin main

echo "==> Rebasing $current_branch onto main"
git checkout "$current_branch"
git rebase main

echo
echo "Done. If you want to update origin/$current_branch:"
echo "  git push --force-with-lease origin $current_branch"
