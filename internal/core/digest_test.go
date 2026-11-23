package core

import "testing"

func TestDigestStringsDeterministicAndOrderIndependent(t *testing.T) {
	a := []string{"/b", "/a", "/c"}
	b := []string{"/c", "/a", "/b"}
	if DigestStrings(a) != DigestStrings(b) {
		t.Fatal("digest must not depend on order")
	}
	if a[0] != "/b" {
		t.Fatal("input slice must not be reordered")
	}
	if DigestStrings(a) == DigestStrings([]string{"/a", "/b"}) {
		t.Fatal("different sets must differ")
	}
	if len(DigestStrings(nil)) != 64 {
		t.Fatal("empty set still yields a sha256 hex digest")
	}
}
