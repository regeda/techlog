package sqlite

import "testing"

func TestFTSQuery(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"   ":              "",
		"spin":             `"spin"*`,
		"promo-api spin":   `"promo-api"* "spin"*`,
		"(":                "",
		"- ( )":            "",
		`say "hi"`:         `"say"* """hi"""*`,
		"AND":              `"AND"*`,
		"incident (opened": `"incident"* "(opened"*`,
	}
	for in, want := range cases {
		if got := ftsQuery(in); got != want {
			t.Errorf("ftsQuery(%q) = %q, want %q", in, got, want)
		}
	}
}
