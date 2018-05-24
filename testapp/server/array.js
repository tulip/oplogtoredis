import { Meteor } from 'meteor/meteor'
import arrayTestCollection from '../imports/api/arrayTest.js';
import insertIgnoreDupKey from '../imports/api/insertIgnoreDupKey.js';

// For testing array modification
Meteor.publish('arrayTest.pub', function() {
  return arrayTestCollection.find();
});

function initializeFixtures() {
  insertIgnoreDupKey(arrayTestCollection, {
    _id: 'test',
    ary: [
      { filter: 10, val: 0 },
      { filter: 20, val: 0 },
      { filter: 30, val: 0 },
      { filter: 40, val: 0 },
    ],
  })

  insertIgnoreDupKey(arrayTestCollection, {
    _id: 'test2',
    ary: [
      { filter: 0, val: 0 },
      { filter: 10, val: 0 },
      { filter: 20, val: 0 },
      { filter: 30, val: 0 },
    ],
  })
}

Meteor.startup(initializeFixtures)

Meteor.methods({
  'arrayTest.initializeFixtures': initializeFixtures,

  'arrayTest.increment'() {
    arrayTestCollection.update({
      'ary.filter': 20,
    }, {
      $inc: {
        'ary.$.val': 1,
      },
    }, { multi: true });
  },
});
