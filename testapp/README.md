# Simple Todo List

The Meteor Tutorial app. Used here to test integration between oplogtoredis
and redis-oplog. When using the oplogtoredis dev environment, you'll have
2 copies of this app running (pointing to the same Mongo and Redis), one
on port 9090 and one on 9091. When working correctly, changes made by
a client on one port should show up in real-time to clients on the other
port.
