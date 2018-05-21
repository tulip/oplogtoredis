import { Meteor } from 'meteor/meteor'
import objectIDTestCollection from '../imports/api/objectIDTest.js';


// For testing object id behavior
const objectIDHex = '5ae7d0042b2acc1f1796c0b6';
const objectID = new Mongo.Collection.ObjectID(objectIDHex);

Meteor.publish('objectIDTest.pub', function() {
  return objectIDTestCollection.find({
    _id: objectID,
  });
})

Meteor.startup(() => {
  if (!objectIDTestCollection.findOne({
      _id: objectID,
  })) {
    try {
        objectIDTestCollection.insert({
        _id: objectID,
        value: 0,
        })
    } catch (e) {
        if (e.clode === 11000) {
            // Ignore -- it was a duplicate key error; some other server just
            // beat us to the insert
            return;
        } else {
            throw e;
        }
    }

    // add a few distractors
    objectIDTestCollection.insert({
        _id: new Mongo.Collection.ObjectID('5ae7d0042b2acc1fdeadbeef'),
        value: 1000,
    })

    objectIDTestCollection.insert({
        _id: 'somestring',
        value: 2000,
    })
  }
})

Meteor.methods({
  'objectIDTest.increment'() {
    objectIDTestCollection.update({
      _id: objectID,
    }, {
      $inc: {
        value: 1,
      }
    })
  }
})
