package shared

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateIDBasedOnContent(t *testing.T) {
	type args struct {
		ct string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Generate ID based on content",
			args: args{
				ct: "test",
			},
			want: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateIDBasedOnContent(tt.args.ct); got != tt.want {
				t.Errorf("GenerateIDBasedOnContent() = %v, want %v", got, tt.want)
			}

			got := GenerateIDBasedOnContent(tt.args.ct)
			assert.Len(t, got, 64)
			assert.True(t, strings.HasPrefix(got, "9f86d081884c7d65"))
		})
	}
}

func TestGenerateUUID(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "Generate UUID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateUUID(); got == "" {
				t.Errorf("GenerateUUID() = %v", got)
			}
		})
	}
}

func TestFindIDField(t *testing.T) {
	type Common struct {
		ID int `json:"id"`
	}

	type X struct {
		Common
		Description string
	}

	type Y struct {
		Name string
		X
	}

	x := Y{
		Name: "test",
		X: X{
			Common:      Common{ID: 123},
			Description: "test",
		},
	}

	if ExtractID(x, "") != "123" {
		t.Fatal("ExtractID() = 123")
	}
}

func TestFindIDField_pointer(t *testing.T) {
	type Common struct {
		ID int `json:"id"`
	}

	type X struct {
		*Common
		Description string
	}

	type Y struct {
		Name string
		*X
	}

	x := &Y{
		Name: "test",
		X: &X{
			Common:      &Common{ID: 123},
			Description: "test",
		},
	}

	if ExtractID(x, "") != "123" {
		t.Fatal("ExtractID() = 123")
	}
}

func TestFindIDField_IDDifferent(t *testing.T) {
	type Common struct {
		ASD int `json:"id"`
	}

	type X struct {
		Common
		Description string
	}

	type Y struct {
		Name string
		X
	}

	x := Y{
		Name: "test",
		X: X{
			Common:      Common{ASD: 123},
			Description: "test",
		},
	}

	if ExtractID(x, "ASD") != "123" {
		t.Fatal("ExtractID() = 123")
	}
}

func TestFindIDField_IDNotFound(t *testing.T) {
	type Common struct {
		ASD int `json:"id"`
	}

	type X struct {
		Common
		Description string
	}

	type Y struct {
		Name string
		X
	}

	x := Y{
		Name: "test",
		X: X{
			Common:      Common{ASD: 123},
			Description: "test",
		},
	}

	if ExtractID(x, "QWE") != "" {
		t.Fatal("ExtractID() = ")
	}
}

func TestFlatten2D(t *testing.T) {
	type args struct {
		data [][]int
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		{
			name: "Should work",
			args: args{
				data: [][]int{
					{1, 2, 3},
					{4, 5, 6},
				},
			},
			want: []int{1, 2, 3, 4, 5, 6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Flatten2D(tt.args.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Flatten2D() = %v, want %v", got, tt.want)
			}
		})
	}
}
