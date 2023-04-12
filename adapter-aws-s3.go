package etler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3Adapter is an adapter for reading and writing data to and from Amazon S3.
type S3Adapter[C any] struct {
	Bucket string
	Key    string
	Region string
}

// Read reads data from Amazon S3.
func (a *S3Adapter[C]) Read(ctx context.Context) ([]C, error) {
	// Create a new AWS session.
	sess, err := session.NewSession(&aws.Config{Region: aws.String(a.Region)})
	if err != nil {
		return nil, err
	}

	// Create a new S3 client.
	client := s3.New(sess)

	// Get the object from S3.
	input := &s3.GetObjectInput{
		Bucket: aws.String(a.Bucket),
		Key:    aws.String(a.Key),
	}
	output, err := client.GetObject(input)
	if err != nil {
		return nil, err
	}

	// Unmarshal the object into a slice of the specified type.
	var data []C
	err = json.NewDecoder(output.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Upsert upserts data into Amazon S3.
func (a *S3Adapter[C]) Upsert(ctx context.Context, data []C) error {
	// Create a new AWS session.
	sess, err := session.NewSession(&aws.Config{Region: aws.String(a.Region)})
	if err != nil {
		return err
	}

	// Create a new S3 client.
	client := s3.New(sess)

	// Marshal the data into JSON.
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Create an object in S3.
	input := &s3.PutObjectInput{
		Bucket: aws.String(a.Bucket),
		Key:    aws.String(a.Key),
		Body:   aws.ReadSeekCloser(io.NopCloser(bytes.NewReader(b))),
	}

	_, err = client.PutObject(input)
	if err != nil {
		return err
	}

	return nil
}
