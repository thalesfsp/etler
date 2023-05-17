package shared

import (
	"reflect"
	"testing"
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
			want: "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateIDBasedOnContent(tt.args.ct); got != tt.want {
				t.Errorf("GenerateIDBasedOnContent() = %v, want %v", got, tt.want)
			}
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
