# mongourl

A utility for parsing Mongo connection URLs. We use this instead of the
built-in parser in `mgo`, becuase the one in `mgo` doesn't support the `?ssl=true`
option. In addition, it applies some reasonable defaults for mgo-specific
options (like Timeout), that aren't part of the Mongo URL spec (because
they're mgo-specific).

This is mostly based on code from Hashicorp's Vault (licensed under MPL 2.0).
