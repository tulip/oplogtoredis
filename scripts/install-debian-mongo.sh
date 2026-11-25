#!/usr/bin/env bash
set -euo pipefail
apt-get install -y wget gnupg 

wget -qO - https://pgp.mongodb.com/server-7.0.asc | gpg -o /usr/share/keyrings/mongodb-server-7.0.gpg --dearmor

echo "deb [ signed-by=/usr/share/keyrings/mongodb-server-7.0.gpg ] https://repo.mongodb.org/apt/debian bookworm/mongodb-org/7.0 main" > /etc/apt/sources.list.d/mongodb-org-7.0.list

apt-get update
# mongodb-org-shell is an empty transitional package from 6.0 onward - it
# installs no binaries at all - so the legacy `mongo` shell is gone. mongosh
# replaces it and is versioned on its own track, not with the server.
apt-get install -y mongodb-org-server=7.0.48 mongodb-mongosh=2.14.2

echo "mongodb-org-server hold" | dpkg --set-selections
echo "mongodb-mongosh hold" | dpkg --set-selections
