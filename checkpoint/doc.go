// Package checkpoint records per-chunk outcomes so a chunked run can keep the
// chunks it finished, record the ones that failed, and resume later with only
// the failed or missing chunks.
//
// pipeline.RunChunks is the consumer. Store is the extension point: Memory and
// File ship with the library, and callers can supply their own, for example
// one backed by a database table keyed by run ID and chunk key.
package checkpoint
