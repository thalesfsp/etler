package option

import (
	"context"
)

//////
// Consts, vars and types.
//////

// Func allows to specify message's options.
type Func func(q Option) Option

// OnErrorFunc is the function that is called when a document fails to be
// stored.
//
// NOTE: Not all adapters support this.
type OnErrorFunc func(ctx context.Context, documentID string, err error)

// OnSuccessFunc is the function that is called when a document is successfully
// stored.
//
// NOTE: Not all adapters support this.
type OnSuccessFunc func(ctx context.Context, documentID string, document string)

//////
// Option definition.
//////

// Option definition.
type Option struct {
	// Body where data can be read from or written to.
	Body any `json:"body,omitempty"`

	// ID of the document.
	ID string `json:"id,omitempty"`

	// IDFieldName is the name of the field that contains the ID.
	IDFieldName string `json:"id_field_name,omitempty"`

	// Fields to be included in the response.
	Fields []string `json:"fields,omitempty"`

	// FieldToUseForID if specified, the content will used to generate the ID
	// of the document.
	FieldToUseForID string `json:"field_to_use_for_id,omitempty"`

	// Limit is the maximum number of items to return.
	Limit int `json:"limit,omitempty"`

	// Offset is the number of items to skip before starting to collect the
	// result set.
	Offset int `json:"offset,omitempty"`

	// OnErrorFunc is the function that is called when a document fails to be
	// stored.
	//
	// WARN: Ideally, DON'T use this for logging purposes!
	// NOTE: Not all adapters support this.
	OnError OnErrorFunc `json:"-"`

	// OnSuccessFunc is the function that is called when a document is successfully
	// stored.
	//
	// WARN: Ideally, DON'T use this for logging purposes!
	// NOTE: Not all adapters support this.
	OnSuccess OnSuccessFunc `json:"-"`

	// Query to be executed.
	Query string `json:"query,omitempty"`

	// Sort field(s) by order.
	Sort map[string]string `json:"sort,omitempty"`

	// Target of the read operation, could be a database, a collection, if any.
	Target string `json:"target,omitempty"`
}

//////
// Factory.
//////

// New returns a new Option with default values.
func New() Option {
	return Option{
		Limit:     10,
		Offset:    0,
		OnSuccess: func(ctx context.Context, documentID string, document string) {},
		OnError:   func(ctx context.Context, documentID string, err error) {},
	}
}

//////
// Built-in options.
//////

// WithBody allows to specify the body where data can be read from or written
func WithBody(body any) Func {
	return func(o Option) Option {
		o.Body = body
		return o
	}
}

// WithID allows to specify the ID of the document.
func WithID(id string) Func {
	return func(o Option) Option {
		o.ID = id
		return o
	}
}

// WithIDFieldName allows to specify the name of the field that contains the ID.
func WithIDFieldName(name string) Func {
	return func(o Option) Option {
		o.IDFieldName = name
		return o
	}
}

// WithFields allows to specify the fields to be included in the response.
func WithFields(fields []string) Func {
	return func(o Option) Option {
		o.Fields = fields
		return o
	}
}

// WithFieldToUseForID allows to specify the field to be used to generate the
func WithFieldToUseForID(field string) Func {
	return func(o Option) Option {
		o.FieldToUseForID = field
		return o
	}
}

// WithLimit allows to specify the maximum number of items to return.
func WithLimit(limit int) Func {
	return func(o Option) Option {
		o.Limit = limit
		return o
	}
}

// WithOffset allows to specify the number of items to skip before starting to
func WithOffset(offset int) Func {
	return func(o Option) Option {
		o.Offset = offset
		return o
	}
}

// WithOnError allows to specify the function that is called when a document
// fails to be stored.
//
// WARN: Ideally, DON'T use this for logging purposes!
// NOTE: Not all adapters support this.
func WithOnError(onError OnErrorFunc) Func {
	return func(o Option) Option {
		o.OnError = onError
		return o
	}
}

// WithOnSuccess allows to specify the function that is called when a document
// is successfully stored.
//
// WARN: Ideally, DON'T use this for logging purposes!
// NOTE: Not all adapters support this.
func WithOnSuccess(onSuccess OnSuccessFunc) Func {
	return func(o Option) Option {
		o.OnSuccess = onSuccess
		return o
	}
}

// WithQuery allows to specify the query to be executed.
func WithQuery(q string) Func {
	return func(o Option) Option {
		o.Query = q
		return o
	}
}

// WithSort allows to specify the sort field(s) by order.
func WithSort(sort map[string]string) Func {
	return func(o Option) Option {
		o.Sort = sort
		return o
	}
}

// WithTarget allows to specify the target of the read operation, could be a
func WithTarget(target string) Func {
	return func(o Option) Option {
		o.Target = target
		return o
	}
}
