package sources

import (
	"strings"
	"testing"
)

func TestIsShorthand(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// Real shorthand
		{"gh:org/repo", true},
		{"gh:org/repo#v1", true},
		{"gl:foo/bar", true},
		{"bb:x/y", true},
		{"  gh:org/repo  ", true}, // whitespace tolerated

		// NOT shorthand
		{"https://github.com/org/repo.git", false},
		{"git@github.com:org/repo.git", false}, // SSH form; not our prefix
		{"/abs/path", false},
		{"./rel", false},
		{"unknown:org/repo", false}, // unrecognized prefix
		{"gh:", false},              // no path
		{"gh://etc", false},         // looks like URL, not shorthand
		{"", false},
	}
	for _, tc := range cases {
		got := IsShorthand(tc.in)
		if got != tc.want {
			t.Errorf("IsShorthand(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseShorthandValid(t *testing.T) {
	cases := []struct {
		in      string
		wantURL string
		wantRef string
	}{
		{"gh:rust-lang/regex", "https://github.com/rust-lang/regex.git", ""},
		{"gh:rust-lang/regex#main", "https://github.com/rust-lang/regex.git", "main"},
		{"gh:rust-lang/regex#v1.10.3", "https://github.com/rust-lang/regex.git", "v1.10.3"},
		{"gh:rust-lang/regex#abc1234", "https://github.com/rust-lang/regex.git", "abc1234"},
		{"gl:gitlab-org/gitlab", "https://gitlab.com/gitlab-org/gitlab.git", ""},
		{"gl:gitlab-org/gitlab#15-0", "https://gitlab.com/gitlab-org/gitlab.git", "15-0"},
		{"bb:atlassian/python-bitbucket", "https://bitbucket.org/atlassian/python-bitbucket.git", ""},
		{"bb:atlassian/python-bitbucket#main", "https://bitbucket.org/atlassian/python-bitbucket.git", "main"},
		// Nested paths (some GitHub repos have group/team prefixes via redirects)
		{"gh:org/repo/sub", "https://github.com/org/repo/sub.git", ""},
	}
	for _, tc := range cases {
		got, err := ParseShorthand(tc.in)
		if err != nil {
			t.Errorf("ParseShorthand(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got.URL != tc.wantURL {
			t.Errorf("ParseShorthand(%q).URL = %q, want %q", tc.in, got.URL, tc.wantURL)
		}
		if got.Ref != tc.wantRef {
			t.Errorf("ParseShorthand(%q).Ref = %q, want %q", tc.in, got.Ref, tc.wantRef)
		}
	}
}

func TestParseShorthandErrors(t *testing.T) {
	cases := []struct {
		in       string
		wantSubs string // substring expected in the error message
	}{
		{"", "empty"},
		{"gh:", "no path"},
		{"gh:#main", "empty path"},
		{"gh:org-only", "org/repo"}, // missing slash
		{"unknown:org/repo", "unknown shorthand prefix"},
		{"no-colon", "missing ':'"},
		{"gh:org /repo", "invalid characters"}, // whitespace in path
	}
	for _, tc := range cases {
		_, err := ParseShorthand(tc.in)
		if err == nil {
			t.Errorf("ParseShorthand(%q) returned nil error; expected failure", tc.in)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantSubs) {
			t.Errorf("ParseShorthand(%q) error = %q, expected substring %q", tc.in, err.Error(), tc.wantSubs)
		}
	}
}

// TestNewSourceAcceptsShorthand verifies that NewSource integrates the
// shorthand parser — passing "gh:org/repo#tag" produces a Source with the
// expanded URL and the ref populated.
func TestNewSourceAcceptsShorthand(t *testing.T) {
	cases := []struct {
		location string
		wantURL  string
		wantRef  string
	}{
		{"gh:co/team-source", "https://github.com/co/team-source.git", ""},
		{"gh:co/team-source#v1.2.0", "https://github.com/co/team-source.git", "v1.2.0"},
		{"gl:group/repo#main", "https://gitlab.com/group/repo.git", "main"},
	}
	for _, tc := range cases {
		src, err := NewSource("team", tc.location)
		if err != nil {
			t.Errorf("NewSource(%q) error: %v", tc.location, err)
			continue
		}
		if src.Kind != KindRemote {
			t.Errorf("NewSource(%q).Kind = %q, want %q", tc.location, src.Kind, KindRemote)
		}
		if src.URL != tc.wantURL {
			t.Errorf("NewSource(%q).URL = %q, want %q", tc.location, src.URL, tc.wantURL)
		}
		if src.Ref != tc.wantRef {
			t.Errorf("NewSource(%q).Ref = %q, want %q", tc.location, src.Ref, tc.wantRef)
		}
	}
}

// TestNewSourcePlainURLPathDoesNotSetRef verifies that the existing long
// form still works and never sets Ref (refs come from --branch/--tag/pin
// today; shorthand is the only way to set Source.Ref).
func TestNewSourcePlainURLDoesNotSetRef(t *testing.T) {
	src, err := NewSource("acme", "https://github.com/acme/registry.git")
	if err != nil {
		t.Fatal(err)
	}
	if src.URL != "https://github.com/acme/registry.git" {
		t.Errorf("URL = %q", src.URL)
	}
	if src.Ref != "" {
		t.Errorf("Ref should be empty for plain URL; got %q", src.Ref)
	}
}
