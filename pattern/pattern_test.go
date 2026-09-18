package pattern

import (
	"reflect"
	"strings"
	"testing"
)

func TestSOCKS5CoversMethodCounts1Through9(t *testing.T) {
	want := make([]string, 0, 9)
	for nmethods := byte(1); nmethods <= 9; nmethods++ {
		want = append(want, string([]byte{0x05, nmethods}))
	}
	if got := Pattern[SOCKS5]; !reflect.DeepEqual(got, want) {
		t.Fatalf("Pattern[SOCKS5] = %q; want %q", got, want)
	}
}

func TestPatternsNonEmpty(t *testing.T) {
	for key, prefixes := range Pattern {
		if len(prefixes) == 0 {
			t.Errorf("Pattern[%s] has no prefixes", key)
		}
		for _, prefix := range prefixes {
			if prefix == "" {
				t.Errorf("Pattern[%s] contains an empty prefix", key)
			}
		}
	}
}

func TestPatternsDoNotOverlap(t *testing.T) {
	type owned struct{ key, prefix string }
	var all []owned
	for key, prefixes := range Pattern {
		for _, prefix := range prefixes {
			all = append(all, owned{key, prefix})
		}
	}
	for i, first := range all {
		for _, second := range all[i+1:] {
			if strings.HasPrefix(first.prefix, second.prefix) || strings.HasPrefix(second.prefix, first.prefix) {
				t.Errorf("%s %q overlaps %s %q", first.key, first.prefix, second.key, second.prefix)
			}
		}
	}
}
