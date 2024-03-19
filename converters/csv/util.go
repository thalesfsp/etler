package csv

import (
	"regexp"
)

//////
// Consts, vars and types.
//////

// Compile the regular expression
var pattern = regexp.MustCompile(`\t`)
