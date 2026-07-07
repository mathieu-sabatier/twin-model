package nodeset

import "testing"

func TestEmbeddedSpecsParse(t *testing.T) {
	for _, s := range Registry() {
		f, err := openSpec(s.File)
		if err != nil {
			t.Fatalf("open %s: %v", s.File, err)
		}
		ns, err := Parse(f)
		f.Close()
		if err != nil {
			t.Fatalf("parse %s: %v", s.File, err)
		}
		if len(ns.Models) == 0 || ns.Models[0].ModelURI != s.URI {
			t.Errorf("%s: model uri = %v, want %s", s.File, ns.Models, s.URI)
		}
	}
}
