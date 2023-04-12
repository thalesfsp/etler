package shared

import (
	"testing"
)

type testStruct struct {
	_ID     string
	ID      string
	IDInt   int
	IDFloat float64
	Name    string
}

func TestExtractID(t *testing.T) {
	tests := []struct {
		name         string
		v            interface{}
		idFieldNames []string
		want         string
	}{
		{
			name: "Extract ID from struct field",
			v: testStruct{
				ID:   "1",
				Name: "John",
			},
			idFieldNames: []string{"ID"},
			want:         "1",
		},
		{
			name: "Extract _ID from struct field with custom name",
			v: testStruct{
				_ID:  "2",
				Name: "Jane",
			},
			idFieldNames: []string{"_ID"},
			want:         "2",
		},
		{
			name: "Extract IDInt from struct field with custom name and int",
			v: testStruct{
				IDInt: 3,
				Name:  "Jane",
			},
			idFieldNames: []string{"IDInt"},
			want:         "3",
		},
		{
			name: "Extract IDFloat from struct field with custom name and float",
			v: testStruct{
				IDFloat: 3.4,
				Name:    "Jane",
			},
			idFieldNames: []string{"IDFloat"},
			want:         "3.4",
		},
		{
			name: "Return empty string if ID field not found",
			v: testStruct{
				Name: "Mike",
			},
			idFieldNames: []string{"ID"},
			want:         "",
		},
		{
			name: "Return empty string if ID field not found and no ID field names provided",
			v: testStruct{
				Name: "Mike",
			},
			idFieldNames: []string{},
			want:         "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractID(tt.v, tt.idFieldNames...); got != tt.want {
				t.Errorf("ExtractID() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
