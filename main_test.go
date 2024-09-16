package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_findHighlights(t *testing.T) {
	type args struct {
		haystack string
		needles  []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "basic",
			args: args{
				haystack: "hello world",
				needles:  []string{"world"},
			},
			want: "hello \033[1m\033[31mworld\033[0m",
		},
		{
			name: "advanced",
			args: args{
				haystack: "hello world, hello peter",
				needles:  []string{"world", "hello"},
			},
			want: "\033[1m\033[31mhello\033[0m \033[1m\033[31mworld\033[0m, hello peter",
		},
		{
			name: "very advanced",
			args: args{
				haystack: "hElLo World, hello peter",
				needles:  []string{"world", "hello"},
			},
			want: "\033[1m\033[31mhElLo\033[0m \033[1m\033[31mWorld\033[0m, hello peter",
		},
		{
			name: "skip overlaps",
			args: args{
				haystack: "Foobar",
				needles:  []string{"foo", "oba"},
			},
			want: "\x1b[1m\x1b[43mFoobar\x1b[0m",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findHighlights(tt.args.haystack, tt.args.needles)

			assert.Equal(t, tt.want, got)
		})
	}
}
