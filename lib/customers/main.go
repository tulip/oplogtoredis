package customers

/*
*
This is a NON-PRODUCTION workaround for testing the parallelism.
It will hardcode in a list of customers/mongo databases/namespace prefixes.
Each one will create a separate read/write coroutine and separate tailable mongo cursor.
This means if one db's OTR buffer fills, the others should continue processing.
This has no discovery mechanism and cannot change dynamically at all.
It is (at the moment) solely used for performance and load testing on staging.
*/
func AllCustomers() []string {
	return []string{
		// namespaces used by acceptance tests
		"tests",
		"test",
		"testdb",
		// used when running in hori
		"factory",
		// used for misc config (probably unnecessary)
		"dev",
		"xxx",
		"something",
		// g2 staging sites
		"alex",
		/* TODO */
	}
}
