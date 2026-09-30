package migrate

import _ "embed"

// Extra holds the hand-written DDL that Ent's schema DSL cannot express. Production
// creates it through the generated migration.
//
//go:embed extra.sql
var Extra string
