#!/bin/sh
# Add one commit to a served repository, so a test can prove Othela notices a
# commit pushed after it started. That is the regression path for issue #32,
# where fleet sync resolved a stale local branch and re-checked-out its
# clone-time commit forever. See ADR-0005.
#
# Usage, from the host:
#
#   docker compose -f e2e.compose.yml exec git-server \
#       seed-commit.sh fleet extra/helvilette.yml "$(cat some-manifest.yml)"
#
# Prints the new commit SHA, which is what the caller asserts against Othela's
# fleet_commit log field.
set -eu

if [ $# -lt 3 ]; then
    echo "usage: $0 <repo> <path-within-repo> <content>" >&2
    echo "  repo must be a directory under /git; got $# arguments" >&2
    exit 2
fi

repo="/git/$1"
target="$2"
content="$3"

if [ ! -d "$repo/.git" ]; then
    echo "no repository at $repo; expected one of: $(ls /git)" >&2
    exit 1
fi

cd "$repo"
mkdir -p "$(dirname "$target")"
printf '%s\n' "$content" > "$target"
git add -A
git commit -q -m "seed $target"
git rev-parse HEAD
