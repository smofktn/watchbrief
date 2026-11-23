package domain

import (
	"strings"
	"testing"
)

func TestSourceURL(t *testing.T) {
	for _, value := range []string{"https://example.com/news?q=日本語#発表", "HTTP://EXAMPLE.COM/a%20b", "http://localhost:8080/", "https://127.0.0.1", "https://[::1]:443/", "https://xn--wgv71a119e.jp/", "https://example.com./"} {
		if !ValidSourceURL(value) {
			t.Errorf("valid URL rejected: %q", value)
		}
	}
	for _, value := range []string{"", "/relative", "//example.com", "https:example.com", "ftp://example.com", "javascript:alert(1)", "https://u:p@example.com", "https://@example.com", "https://example.com\\x", "https://example.com/ a", "https://example.com/\n", "https://example.com/\u0085", "https://example.com/%xx", "https://example.com/?x=%", "https://example.com/#%xx", "https://example.com:", "https://example.com:0", "https://example.com:65536", "https://例.jp/", "https://-bad.com", "https://a..b", "https://[v1.test]", "https://[fe80::1%25eth0]", "https://example.com/%+1", "https://example.com/" + strings.Repeat("x", 2048)} {
		if ValidSourceURL(value) {
			t.Errorf("invalid URL accepted: %q", value)
		}
	}
}

func TestSourceChangesValidation(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	valid := SourceChange{WatchTargetID: id, Name: "架空", URL: "https://example.com", Instruction: ""}
	base := CollectionUpdate{NewCollection: NewCollection{Name: "架空", InitialLookbackMode: "ai"}, Version: "1", Sources: []SourceChange{valid}}
	if base.Validate() != nil {
		t.Fatal("valid rejected")
	}
	for _, source := range []SourceChange{
		{WatchTargetID: "bad", Name: "架空", URL: valid.URL},
		{WatchTargetID: id, Name: "　", URL: valid.URL},
		{WatchTargetID: id, Name: strings.Repeat("あ", 201), URL: valid.URL},
		{WatchTargetID: id, Name: "架空", URL: valid.URL, Instruction: strings.Repeat("あ", 10001)},
		{WatchTargetID: id, Name: "架空\x00", URL: valid.URL},
	} {
		base.Sources = []SourceChange{source}
		if base.Validate() == nil {
			t.Fatal("invalid source accepted")
		}
	}
	valid.ID = &id
	base.Sources = []SourceChange{valid, valid}
	if base.Validate() == nil {
		t.Fatal("duplicate accepted")
	}
	valid.ID = nil
	base.Sources = make([]SourceChange, 21)
	for i := range base.Sources {
		base.Sources[i] = valid
	}
	if base.Validate() == nil {
		t.Fatal("21 accepted")
	}
	base.Sources = base.Sources[:20]
	if base.Validate() != nil {
		t.Fatal("20 rejected")
	}
}
