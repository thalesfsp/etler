package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/thalesfsp/etler/adapter"
	"github.com/thalesfsp/etler/option"
	"github.com/thalesfsp/etler/shared"
)

// Config is the ElasticSearch configuration.
type Config = elasticsearch.Config

// ElasticSearch adapter.
type ElasticSearch[C any] struct {
	*adapter.Adapter

	client *elasticsearch.Client
	index  string
}

// Read data from data source.
func (es *ElasticSearch[C]) Read(ctx context.Context, o ...option.Func) ([]C, error) {
	opt := option.New()

	for _, f := range o {
		opt = f(opt)
	}

	// Determine the final index. For situations where the index is dynamic,
	// this covers the case.
	finalIndex := es.index
	if opt.Target != "" {
		finalIndex = opt.Target
	}

	// Build the search options.
	opts := []func(*esapi.SearchRequest){
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex(finalIndex),
	}

	if opt.Limit > 0 {
		opts = append(opts, es.client.Search.WithSize(opt.Limit))
	}

	if opt.Offset > 0 {
		opts = append(opts, es.client.Search.WithFrom(opt.Offset))
	}

	if len(opt.Fields) > 0 {
		// adapter.WithFields([]string{"-Name", "+Age", "+CreatedAt"}),
		// if starts with "-" then it's a field to exclude
		var includeFields []string
		var excludeFields []string

		for _, f := range opt.Fields {
			if strings.HasPrefix(f, "-") {
				excludeFields = append(excludeFields, strings.TrimPrefix(f, "-"))
			} else {
				includeFields = append(includeFields, f)
			}
		}

		if len(includeFields) > 0 {
			opts = append(opts, es.client.Search.WithSourceIncludes(opt.Fields...))
		}

		if len(excludeFields) > 0 {
			opts = append(opts, es.client.Search.WithSourceExcludes(excludeFields...))
		}
	}

	if len(opt.Sort) > 0 {
		// Convert opt.Sort to the way ElasticSearch understands, a list of
		// field>:<direction> pairs.
		var sortSlice []string

		for k, v := range opt.Sort {
			sortSlice = append(sortSlice, fmt.Sprintf("%s:%s", k, v))
		}

		opts = append(opts, es.client.Search.WithSort(sortSlice...))
	}

	if opt.Query != "" {
		opts = append(opts, es.client.Search.WithBody(strings.NewReader(opt.Query)))
	}

	// Execute the search query and retrieves the results.
	res, err := es.client.Search(opts...)
	if err != nil {
		return nil, err
	}

	// Get the HTTP response body.
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search query failed with %s", res.Status())
	}

	// Unmarshal the response.
	var r struct {
		Hits struct {
			Hits []struct {
				Index  string  `json:"_index"`
				ID     string  `json:"_id"`
				Score  float64 `json:"_score"`
				Source C       `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	// Map the response to the desired type.
	var results []C

	for _, hit := range r.Hits.Hits {
		results = append(results, hit.Source)
	}

	return results, nil
}

// Upsert data to data source.
//
// NOTE: It will try to extract the ID from each item in `v`. By the default it
// will look up for a field called `ID` and `_ID`. You can specify a different
// field name using the `adapter.WithIDFieldName` option.
// NOTE: It uses the Bulk API to upsert the data which means some items may be
// upserted and some may not. It's up to the caller to handle this situation.
func (es *ElasticSearch[C]) Upsert(ctx context.Context, v []C, o ...option.Func) error {
	// Should be able to specify options.
	opt := option.New()

	for _, f := range o {
		opt = f(opt)
	}

	// Should be able to specify a dynamic index.
	finalIndex := es.index
	if opt.Query != "" {
		finalIndex = opt.Query
	}

	for _, doc := range v {
		// Should be able to automatically extract the ID, if any.
		// Should be able to specify different ID field name.
		id := shared.ExtractID(doc, opt.IDFieldName)

		if id == "" {
			if opt.FieldToUseForID != "" {
				// Should be able to specify the field's content to use for ID.
				id = shared.GenerateIDBasedOnContent(shared.ExtractID(doc, opt.FieldToUseForID))
			} else {
				// Should generate a a RFC4122 UUID/DCE 1.1 if no ID is found.
				id = shared.GenerateUUID()
			}
		}

		data, err := json.Marshal(doc)
		if err != nil {
			opt.OnError(ctx, id, err)

			return err
		}

		// Store the document.
		if _, err := es.client.Index(
			finalIndex,
			bytes.NewReader(data),
			es.client.Index.WithDocumentID(id),
			es.client.Index.WithRefresh("true"),
		); err != nil {
			opt.OnError(ctx, id, err)

			return err
		}

	}

	// Flush the index.
	if _, err := es.client.Indices.Flush(es.client.Indices.Flush.WithIndex(finalIndex)); err != nil {
		opt.OnError(ctx, "", err)

		return err
	}

	return nil
}

// New returns a new ElasticSearch adapter.
func New[C any](index string, esConfig Config) (adapter.IAdapter[C], error) {
	//////
	// Set default values for the ElasticSearch configuration.
	//////

	if esConfig.RetryOnStatus == nil {
		esConfig.RetryOnStatus = []int{502, 503, 504, 429}
	}

	if esConfig.RetryBackoff == nil {
		retryBackoff := backoff.NewExponentialBackOff()

		esConfig.RetryBackoff = func(i int) time.Duration {
			if i == 1 {
				retryBackoff.Reset()
			}
			return retryBackoff.NextBackOff()
		}
	}

	if esConfig.MaxRetries == 0 {
		esConfig.MaxRetries = 5
	}

	//////
	// Create the ElasticSearch client.
	//////

	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating the ElasticSearch client: %w", err)
	}

	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("error pinging Elasticsearch: %w", err)
	}

	if _, err := io.Copy(io.Discard, res.Body); err != nil {
		return nil, fmt.Errorf("error consuming the response body: %w", err)
	}

	// NOTE: It is critical to both close the response body and to consume it,
	// in order to re-use persistent TCP connections in the default HTTP
	// transport. If you're not interested in the response body, call
	// `io.Copy(ioutil.Discard, res.Body).`
	defer res.Body.Close()

	return &ElasticSearch[C]{
		Adapter: adapter.New("elasticsearch", "elasticsearch adapter"),

		client: client,
		index:  index,
	}, nil
}
