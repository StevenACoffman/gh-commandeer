package cmdutil_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/gh-commandeer/pkg/cmdutil"
)

func TestParseKeepList(t *testing.T) {
	t.Parallel()

	defaults := []string{"origin", "upstream", "mine"}

	cases := map[string]struct {
		raw  string
		want []string
	}{
		"empty defaults":      {"", defaults},
		"whitespace defaults": {"   ", defaults},
		"single value":        {"foo", []string{"foo"}},
		"comma separated":     {"a,b,c", []string{"a", "b", "c"}},
		"trims whitespace":    {"  a  , b ,c  ", []string{"a", "b", "c"}},
		"drops empty entries": {"a,,b,", []string{"a", "b"}},
		// ",,," is a non-empty string per the user-facing contract: explicit input
		// is honored verbatim, so the result is the empty keep-list and every remote
		// becomes a deletion candidate. The default only substitutes for "" / blanks.
		"explicit empty result": {",,,", []string{}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := cmdutil.ParseKeepList(tc.raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseKeepList(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestConfirm(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		input string
		want  bool
	}{
		"y":              {"y\n", true},
		"yes":            {"yes\n", true},
		"Y":              {"Y\n", true},
		"YES":            {"YES\n", true},
		"yes with space": {"  yes  \n", true},
		"n":              {"n\n", false},
		"no":             {"no\n", false},
		"empty line":     {"\n", false},
		"eof":            {"", false},
		"other":          {"maybe\n", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			got, err := cmdutil.Confirm(strings.NewReader(tc.input), &out, "Proceed?")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Confirm(%q) = %v, want %v", tc.input, got, tc.want)
			}
			if !strings.Contains(out.String(), "Proceed? [y/N]: ") {
				t.Errorf("prompt missing from output: %q", out.String())
			}
		})
	}
}
