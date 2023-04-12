package etler

// import (
// 	"context"
// 	"encoding/xml"
// 	"io"
// )

// // XMLAdapter is an adapter for reading and writing data in XML format.
// type XMLAdapter [C any]struct {
// 	Reader io.Reader
// 	Writer io.Writer
// }

// // Read reads data from an XML document.
// func (a *XMLAdapter[C any]) Read(ctx context.Context) ([]C, error) {
// 	// Unmarshal the XML into a slice of the specified type.
// 	var data []C
// 	err := xml.NewDecoder(a.Reader).Decode(&data)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return data, nil
// }

// // Upsert writes data to an XML document.
// func (a *XMLAdapter[C any]) Upsert(ctx context.Context, data []C) error {
// 	// Marshal the data into XML.
// 	err := xml.NewEncoder(a.Writer).Encode(data)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
