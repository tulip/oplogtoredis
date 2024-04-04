package denylist

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// KeysSeparator is the character that separates the object keys when parsing a new denylist entry
const KeysSeparator = "."

// DenylistEntry is one active rule for when an oplog update would be skipped
type DenylistEntry struct {
	// Keys is an array of object keys to index into an oplog update document.
	// Each key is applied to the object at the previous key, so to index the text in an object
	// like {"a":{"b":{"c":"text"}}}, the Keys array would be ["a", "b", "c"]
	Keys []string
	// Regex is a regular expression to test against the oplog document's data at the specified index.
	// If the regex is found anywhere in the string, the update document will be skipped.
	// For instance, to skip the above document if it contained exactly the string text, the regex could be
	// `^text$`
	Regex *regexp.Regexp
}

// Denylist is a list of rules for skipping oplog updates
type Denylist map[string]*DenylistEntry

// NewDenylist creates a new empty Denylist with no rules
func NewDenylist() *Denylist {
	return &Denylist{}
}

// GetKeys returns a list of identifiers for the active rules of this Denylist
func (dl *Denylist) GetKeys() []string {
	keys := make([]string, len(*dl))

	i := 0
	for k := range *dl {
		keys[i] = k
		i++
	}

	return keys
}

// GetEntry returns an active Denylist rule corresponding to the provided identifier
func (dl *Denylist) GetEntry(key string) *DenylistEntry {
	if dle, ok := (*dl)[key]; ok {
		return dle
	}
	return nil
}

// DeleteEntry removes a rule from this Denylist, so it will no longer cause oplog updates to be skipped.
// Returns true if the rule existed (and was removed), or false if it didn't (and thus wasn't).
func (dl *Denylist) DeleteEntry(key string) bool {
	if _, ok := (*dl)[key]; ok {
		delete(*dl, key)
		return true
	}
	return false
}

// AppendEntry constructs and adds a new rule to this Denylist. The contents of the rule (keys to check and regex)
// are provided as strings. The unparsed keys array should be delimited by the keysSeparator character.
// Returns the random identifier for the new rule in the Denylist, or an error if the regex couldn't be compiled.
func (dl *Denylist) AppendEntry(unparsedKeys string, unparsedRegex string) (string, error) {
	keys := strings.Split(unparsedKeys, KeysSeparator)
	regex, err := regexp.Compile(unparsedRegex)
	if err != nil {
		return "", errors.Wrap(err, "parsing denylist regex")
	}

	entryKey := uuid.New().String()
	(*dl)[entryKey] = &DenylistEntry{
		Keys:  keys,
		Regex: regex,
	}

	return entryKey, nil
}

// PassFilter tests if a provided object passes this Denylist rule.
// First, the keys are used to index into the object. If, for any key, the
// object is not a map that can be indexed, the object automatically _passes_ the filter.
// Then, if the object at the indexed location is not a string, it automatically _passes_ the filter.
// Otherwise, it will fail the filter if the regex could be found somewhere in the string, otherwise it passes.
func (dle *DenylistEntry) PassFilter(obj interface{}) bool {
	for _, key := range dle.Keys {
		if mapObj, ok := obj.(map[string]interface{}); ok {
			obj = mapObj[key]
		} else {
			return true
		}
	}
	if str, ok := obj.(string); ok {
		return !dle.Regex.MatchString(str)
	} else {
		return true
	}
}

// Filter tests if a provided object passes every Denylist rule.
// If it fails any rule, it fails the entire Denylist, and returns the ID of that rule.
// If it passes every route, it passes the list, and returns the empty string.
func (dl *Denylist) Filter(obj map[string]interface{}) string {
	for id, dle := range *dl {
		if !dle.PassFilter(obj) {
			return id
		}
	}
	return ""
}
