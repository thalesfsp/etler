package adapter

import (
	"context"

	"github.com/thalesfsp/etler/option"
	"github.com/thalesfsp/validation"
)

// Adapter definition.
type Adapter struct {
	// Name of the adapter.
	Name string `json:"name" validate:"required"`

	// Description of the adapter.
	Description string `json:"description"`
}

// IDAO defines how to read and upsert data.
type IDAO[C any] interface {
	// Read from data source.
	Read(ctx context.Context, o ...option.Func) ([]C, error)

	// Upsert write to the data source.
	Upsert(ctx context.Context, v []C, o ...option.Func) error
}

// IAdapter defines what an `Adapter` must do.
type IAdapter[C any] interface {
	// GetDescription returns the `Description` of the `Adapter`.
	GetDescription() string

	// GetName returns the `Nane` of the adapter.
	GetName() string

	IDAO[C]
}

// GetDescription returns the `Description` of the `Adapter`.
func (a *Adapter) GetDescription() string {
	return a.Description
}

// GetName returns the `Nane` of the adapter.
func (a *Adapter) GetName() string {
	return a.Name
}

// New creates a new `Adapter`.
func New(name, description string) (*Adapter, error) {
	a := &Adapter{
		Name:        name,
		Description: description,
	}

	if err := validation.Validate(a); err != nil {
		return nil, err
	}

	return a, nil
}
