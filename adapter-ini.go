package etler

// import (
// 	"context"
// 	"encoding/json"
// 	"io"

// 	"github.com/go-ini/ini"
// )

// // INIAdapter is an adapter for reading and writing data in INI format.
// type INIAdapter [C any]struct {
// 	Reader io.Reader
// 	Writer io.Writer
// }

// // Read reads data from an INI document.
// func (a *INIAdapter[C any]) Read(ctx context.Context) ([]C, error) {
// 	// Read the INI from the reader.
// 	cfg, err := ini.Load(a.Reader)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Get the value of the specified key.
// 	val := cfg.Section("").Key("key").Value()

// 	// Unmarshal the value into a slice of the specified type.
// 	var result []C
// 	err = json.Unmarshal([]byte(val), &result)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// // Upsert writes data to an INI document.
// func (a *INIAdapter[C any]) Upsert(ctx context.Context, data []C) error {
// 	// Marshal the data into JSON.
// 	val, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}

// 	// Create a new INI document.
// 	cfg := ini.Empty()

// 	// Set the value of the specified key.
// 	cfg.Section("").Key("key").SetValue(string(val))

// 	// Write the INI to the writer.
// 	err = cfg.SaveTo(a.Writer)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
