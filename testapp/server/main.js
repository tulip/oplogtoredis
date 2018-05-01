import { Meteor } from 'meteor/meteor'
import { Tasks } from '../imports/api/tasks.js';
import arrayTestCollection from '../imports/api/arrayTest.js';


Foo = new Mongo.Collection('Foo')

// For testing object id behavior
Foo.find({
  _id: new Mongo.Collection.ObjectID('5ae7d0042b2acc1f1796c0b6')
}).observe({
  changed(newDoc) {
    console.log('CHANGE', newDoc)
  }
})


// For testing array modification
Meteor.publish('arrayTest.pub', function() {
  if (arrayTestCollection.find({ _id: 'test' }).count() < 1) {
    arrayTestCollection.insert({
      _id: 'test',
      ary: [
        { filter: 10, val: 0 },
        { filter: 20, val: 0 },
        { filter: 30, val: 0 },
        { filter: 40, val: 0 },
      ],
    });
  }

  return arrayTestCollection.find();
});


Meteor.methods({
  'arrayTest.increment'() {
    arrayTestCollection.update({
      _id: 'test',
      'ary.filter': 20,
    }, {
      $inc: {
        'ary.$.val': 1,
      },
    });
  },
});
