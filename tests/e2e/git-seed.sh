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

# In this setup, we rely on docker-compose executing the gitea admin command directly
# before git-seeder pushes. Alternatively, if no user exists, the push will fail.
# For a fully automated mock, we would use an API or pre-created database.
# Here we just try to push assuming the user exists (or the repo is public).

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
git config --global user.email "$USER@helvilette.local"
git config --global user.name "$USER"
git config --global init.defaultBranch main

if [ ! -d .git ]; then
    git init
    git add .
    git commit -m "Initial commit from seed"
fi

git remote remove origin 2>/dev/null || true
git remote add origin "http://$USER:$PASS@git-server:3000/$USER/$REPO_NAME.git"
git branch -M main
git push -u origin main -f || echo "Failed to push to Gitea. Make sure the user exists."

echo "Seeding completed successfully!"
# Keep container running
tail -f /dev/null