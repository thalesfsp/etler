package etler

// import (
// 	"context"
// 	"io"
// 	"io/ioutil"

// 	"github.com/pelletier/go-toml"
// )

// // TOMLAdapter is an adapter for reading and writing data in TOML format.
// type TOMLAdapter [C any]struct {
// 	Reader io.Reader
// 	Writer io.Writer
// }

// // Read reads data from a TOML document.
// func (a *TOMLAdapter[C any]) Read(ctx context.Context) ([]C, error) {
// 	// Read the TOML from the reader.
// 	data, err := ioutil.ReadAll(a.Reader)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Unmarshal the TOML into a slice of the specified type.
// 	var result []C
// 	err = toml.Unmarshal(data, &result)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// // Upsert writes data to a TOML document.
// func (a *TOMLAdapter[C any]) Upsert(ctx context.Context, data []C) error {
// 	// Marshal the data into TOML.
// 	out, err := toml.Marshal(data)
// 	if err != nil {
// 		return err
// 	}

// 	// Write the TOML to the writer.
// 	_, err = a.Writer.Write(out)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
