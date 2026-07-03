package copypath

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildPaths(t *testing.T) {
	sep := string(filepath.Separator)
	join := func(parts ...string) string { return filepath.Join(parts...) }

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "nested file",
			in:   join(sep, "users", "bob", "data", "1.txt"),
			want: []string{
				"1.txt",
				join("data", "1.txt"),
				join("bob", "data", "1.txt"),
				join(sep, "users", "bob", "data", "1.txt"),
			},
		},
		{
			name: "single parent",
			in:   join(sep, "home", "1.txt"),
			want: []string{
				"1.txt",
				join(sep, "home", "1.txt"),
			},
		},
		{
			name: "file at root",
			in:   join(sep, "1.txt"),
			want: []string{
				"1.txt",
				join(sep, "1.txt"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPaths(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildPaths(%q)\n got: %#v\nwant: %#v", tt.in, got, tt.want)
			}
		})
	}
}
