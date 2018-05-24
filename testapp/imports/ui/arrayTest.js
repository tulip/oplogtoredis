import './arrayTest.html';
import arrayTestCollection from '../api/arrayTest.js';

Template.arrayTest.onCreated(function() {
    this.subHandle = Meteor.subscribe('arrayTest.pub');
})

Template.arrayTest.onDestroyed(function() {
    this.subHandle.stop();
})

Template.arrayTest.helpers({
    recordJSON() {
        return JSON.stringify(arrayTestCollection.find().fetch(), null, 4)
    }
})

Template.arrayTest.events({
    'click .increment'() {
        Meteor.call('arrayTest.increment');
    }
})
