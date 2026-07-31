// Package csv converts slices of values into CSV text. Converter construction
// rejects cyclic row types before gocsv can recurse until stack overflow. The
// marshal boundary turns other gocsv panics, including panics from caller
// MarshalCSV methods, into errors. Valid csv[] expansion remains supported.
package csv
