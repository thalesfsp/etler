package etler

// import (
// 	"context"
// 	"io"
// 	"io/ioutil"

// 	"gopkg.in/yaml.v2"
// )

// // YAMLAdapter is an adapter for reading and writing data in YAML format.
// type YAMLAdapter [C any]struct {
// 	Reader io.Reader
// 	Writer io.Writer
// }

// // Read reads data from a YAML document.
// func (a *YAMLAdapter[C any]) Read(ctx context.Context) ([]C, error) {
// 	// Read the YAML from the reader.
// 	data, err := ioutil.ReadAll(a.Reader)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Unmarshal the YAML into a slice of the specified type.
// 	var result []C
// 	err = yaml.Unmarshal(data, &result)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// // Upsert writes data to a YAML document.
// func (a *YAMLAdapter[C any]) Upsert(ctx context.Context, data []C) error {
// 	// Marshal the data into YAML.
// 	out, err := yaml.Marshal(data)
// 	if err != nil {
// 		return err
// 	}

// 	// Write the YAML to the writer.
// 	_, err = a.Writer.Write(out)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
