#!/usr/bin/env bash
apt-get install -y wget gnupg 

wget -qO - https://www.mongodb.org/static/pgp/server-4.4.asc | apt-key add -

echo "deb http://repo.mongodb.org/apt/debian buster/mongodb-org/4.4 main" | tee /etc/apt/sources.list.d/mongodb-org-4.4.list

apt-get update
apt-get install -y mongodb-org=4.4.14 mongodb-org-server=4.4.14 mongodb-org-shell=4.4.14 mongodb-org-tools=4.4.14


