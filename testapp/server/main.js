import { Tasks } from '../imports/api/tasks.js';

Foo = new Mongo.Collection('Foo')
Meteor.publish('newfoo', function() {
  newid = Random.id();

  Meteor.setTimeout(() => {
    Foo.insert({ _id: newid })
  }, 500)

  return Foo.find({ _id: newid });
})
