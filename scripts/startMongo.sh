#!/bin/sh

mongod --fork --logpath /var/log/mongod.log --replSet myapp --port 27017 --bind_ip 127.0.0.1,mongo
mongo --eval 'rs.initiate({ _id: "myapp", members: [{ _id: 0, host: "mongo:27017"}] })'
# we stared mongod in the background and don't want the container to exit
tail -f /dev/null
