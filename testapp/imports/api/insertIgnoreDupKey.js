export default function insertIgnoreDupKey(collection, entry) {
    try {
        collection.insert(entry)
      } catch(e) {
        if (e.code === 11000) {
          // Ignore -- it was a duplicate key error; some other server just
          // beat us to the insert
          return;
        } else {
            throw e;
        }
      }
}
