package dsl

import "testing"

const perspSrc = "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-06 }\n" +
	"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
	"object_types: { FT: { base: OpcUa:BaseObjectType } }\n" +
	"instances:\n" +
	"  Casepacker01: { type: FT, under: OpcUa:ObjectsFolder }\n" +
	"  Palletizer03: { type: FT, under: OpcUa:ObjectsFolder }\n" +
	"perspectives:\n" +
	"  spatial_zones:\n" +
	"    label: \"Spatial / fire zones\"\n" +
	"    membership: exclusive\n" +
	"    export: false\n" +
	"    nodes:\n" +
	"      hall_b:\n" +
	"        label: \"Hall B\"\n" +
	"        children: [zone_b2]\n" +
	"      zone_b2:\n" +
	"        label: \"Zone B2\"\n" +
	"        members: [Casepacker01, Palletizer03]\n"

func TestParsePerspectives(t *testing.T) {
	m, err := Parse("t.yaml", []byte(perspSrc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m.Perspectives) != 1 {
		t.Fatalf("perspectives = %d, want 1", len(m.Perspectives))
	}
	p := m.Perspectives[0]
	if p.ID != "spatial_zones" || p.Membership != "exclusive" || p.Export {
		t.Errorf("perspective header wrong: %+v", p)
	}
	if len(p.Nodes) != 2 || p.Nodes[0].ID != "hall_b" || p.Nodes[0].Children[0] != "zone_b2" {
		t.Fatalf("nodes wrong: %+v", p.Nodes)
	}
	if got := p.Nodes[1].Members; len(got) != 2 || got[0] != "Casepacker01" {
		t.Errorf("members wrong: %v", got)
	}
}

func TestFormatRoundTripsPerspectives(t *testing.T) {
	m, err := Parse("t.yaml", []byte(perspSrc))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Format(m)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := Parse("t.yaml", out)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	if len(m2.Perspectives) != 1 || len(m2.Perspectives[0].Nodes) != 2 ||
		m2.Perspectives[0].Nodes[1].Members[1] != "Palletizer03" {
		t.Errorf("round-trip lost perspective data:\n%s", out)
	}
}

const pHdr = "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-06 }\n" +
	"imports: { OpcUa: http://opcfoundation.org/UA/ }\n" +
	"object_types: { FT: { base: OpcUa:BaseObjectType } }\n" +
	"instances: { A: { type: FT, under: OpcUa:ObjectsFolder }, B: { type: FT, under: OpcUa:ObjectsFolder } }\n"

func TestValidatePerspectives(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"dangling-member",
			pHdr + "perspectives: { p: { nodes: { n: { members: [Nope] } } } }",
			"dangling-member"},
		{"exclusive-membership",
			pHdr + "perspectives:\n  p:\n    membership: exclusive\n    nodes:\n" +
				"      n1: { members: [A] }\n      n2: { members: [A] }\n",
			"exclusive-membership"},
		{"unknown-membership-mode",
			pHdr + "perspectives: { p: { membership: weird, nodes: { n: { members: [A] } } } }",
			"unknown-membership-mode"},
		{"unknown-perspective-node",
			pHdr + "perspectives: { p: { nodes: { n: { children: [ghost] } } } }",
			"unknown-perspective-node"},
		{"perspective-node-cycle",
			pHdr + "perspectives: { p: { nodes: { a: { children: [b] }, b: { children: [a] } } } }",
			"perspective-node-cycle"},
		{"perspective-id-not-exportable",
			pHdr + "perspectives: { \"bad-id\": { export: true, nodes: { n: { members: [A] } } } }",
			"perspective-id-not-exportable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if codes := codesFor(t, c.src); !hasCode(codes, c.want) {
				t.Errorf("want %q; got %v", c.want, codes)
			}
		})
	}
}

func TestValidateOverlappingAllowsDuplicate(t *testing.T) {
	src := pHdr + "perspectives:\n  p:\n    membership: overlapping\n    nodes:\n" +
		"      n1: { members: [A] }\n      n2: { members: [A] }\n"
	if codes := codesFor(t, src); hasCode(codes, "exclusive-membership") {
		t.Errorf("overlapping must allow an instance in two nodes; got %v", codes)
	}
}
