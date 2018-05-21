import { Meteor } from 'meteor/meteor'
import arrayTestCollection from '../imports/api/arrayTest.js';

// For testing array modification
Meteor.publish('arrayTest.pub', function() {
  return arrayTestCollection.find();
});

Meteor.startup(() => {
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
})

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
