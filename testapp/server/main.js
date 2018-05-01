import { Tasks } from '../imports/api/tasks.js';

Foo = new Mongo.Collection('Foo')

// For testing object id behavior
Foo.find({
  _id: new Mongo.Collection.ObjectID('5ae7d0042b2acc1f1796c0b6')
}).observe({
  changed(newDoc) {
    console.log('CHANGE', newDoc)
  }
})
