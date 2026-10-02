#!/bin/sh
# Turn every directory under $1 into its own Git repository under $2, committed.
#
# Run at image build time, not at container start. The bootstrap it replaces was
# a single inline `sh -c 'mkdir && cp && git init && git add && git commit && exec git daemon'`
# written twice, once in the compose file and once in the Ginkgo suite, which had
# to be kept in step by hand. Baking also fixes the served commit SHA, so every
# run starts from the same commit. See ADR-0007 D4.
set -eu

src="$1"
dest="$2"

mkdir -p "$dest"

for path in "$src"/*; do
    [ -d "$path" ] || continue
    name=$(basename "$path")

    repo="$dest/$name"
    mkdir -p "$repo"
    cp -a "$path"/. "$repo"/

    cd "$repo"
    rm -rf .git
    git init -q -b main
    git config user.name "Helvilette e2e"
    git config user.email "e2e@helvilette.invalid"
    # Lets seed-commit.sh push into a non-bare checkout, which is what the #32
    # regression path needs.
    git config receive.denyCurrentBranch ignore
    git add -A
    git commit -q -m "Bake $name fixture"

    echo "baked $name at $(git rev-parse --short HEAD)"
done
