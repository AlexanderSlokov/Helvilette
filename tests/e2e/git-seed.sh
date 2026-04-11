#!/bin/sh
# wait for Gitea to start
sleep 5

# Set up variables
GITEA_URL="http://git-server:3000"
USER="helvilette"
PASS="helvilette123"
REPO_NAME="nginx-collection"

# Function to check if Gitea is up
wait_for_gitea() {
  echo "Waiting for Gitea to be ready..."
  until curl -s -f -o /dev/null "$GITEA_URL/api/swagger"; do
    sleep 2
  done
  echo "Gitea is up!"
}

wait_for_gitea

# Find the git-server container using docker CLI and create the admin user
echo "Creating admin user via docker exec..."
CONTAINER_ID=$(docker ps -qf "name=git-server" | head -n 1)
if [ -n "$CONTAINER_ID" ]; then
    docker exec -u git $CONTAINER_ID gitea admin user create --username $USER --password $PASS --email $USER@helvilette.local --admin || echo "User may already exist"
else
    echo "Warning: git-server container not found by docker cli!"
fi

sleep 2

# Using Basic Auth for the new user directly to create repo
echo "Creating repository $REPO_NAME..."
curl -s -X POST "$GITEA_URL/api/v1/user/repos" \
  -H "accept: application/json" \
  -H "Content-Type: application/json" \
  -u "$USER:$PASS" \
  -d "{\"name\":\"$REPO_NAME\",\"private\":false}" || echo "Repo already exists"

# Initialize local git repo and push
echo "Pushing data to repository..."
cd /data/playbooks/$REPO_NAME || exit 1

# Fix for "fatal: detected dubious ownership in repository"
git config --global --add safe.directory /data/playbooks/$REPO_NAME

git config --global user.email "$USER@helvilette.local"
git config --global user.name "$USER"
git config --global init.defaultBranch main

if [ ! -d .git ]; then
    git init
    git checkout -b main || git branch -M main
    git add .
    git commit -m "Initial commit from seed"
else
    # If git is already initialized, just add new files and commit
    git add .
    git commit -m "Update from seed" || echo "Nothing to commit"
    git branch -M main || git checkout -b main
fi

git remote remove origin 2>/dev/null || true
git remote add origin "http://$USER:$PASS@git-server:3000/$USER/$REPO_NAME.git"
git push -u origin main -f || echo "Failed to push to Gitea. Make sure the user exists."

echo "Seeding completed successfully!"