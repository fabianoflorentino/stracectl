package aggregator

import "testing"

func TestExtractPathFromArgs(t *testing.T) {
	cases := []struct {
		name string
		args string
		want string
	}{
		{"open", "\"/etc/hosts\", O_RDONLY", "/etc/hosts"},
		{"openat", "AT_FDCWD, \"/etc/ld.so.cache\", O_RDONLY", "/etc/ld.so.cache"},
		{"open", "/etc/hosts, O_RDONLY", "/etc/hosts"},
		{"openat", "3, /tmp/foo, O_RDONLY", "/tmp/foo"},
		{"open", "NULL, 0", ""},
	}

	for _, c := range cases {
		got := extractPathFromArgs(c.name, c.args)
		if got != c.want {
			t.Errorf("%s %q: want %q, got %q", c.name, c.args, c.want, got)
		}
	}
}
