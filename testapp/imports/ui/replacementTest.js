import './replacementTest.html';
import replacementTestCollection from '../api/replacementTest.js';

Template.replacementTest.onCreated(function() {
    this.subHandle = Meteor.subscribe('replacementTest.pub');
})

Template.replacementTest.onDestroyed(function() {
    this.subHandle.stop();
})

Template.replacementTest.helpers({
    recordJSON() {
        return JSON.stringify(replacementTestCollection.findOne(), null, 4)
    }
})

Template.replacementTest.events({
    'click .a'() {
        Meteor.call('replacementTest.change', 'a');
    },
    'click .b'() {
        Meteor.call('replacementTest.change', 'b');
    }
})
