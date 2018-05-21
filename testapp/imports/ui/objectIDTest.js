import './objectIDTest.html';
import objectIDCollection from '../api/objectIDTest.js';

Template.objectIDTest.onCreated(function() {
    this.subHandle = Meteor.subscribe('objectIDTest.pub');
})

Template.objectIDTest.onDestroyed(function() {
    this.subHandle.stop();
})

Template.objectIDTest.helpers({
    recordJSON() {
        return JSON.stringify(objectIDCollection.findOne(), null, 4)
    }
})

Template.objectIDTest.events({
    'click .increment'() {
        Meteor.call('objectIDTest.increment');
    }
})
