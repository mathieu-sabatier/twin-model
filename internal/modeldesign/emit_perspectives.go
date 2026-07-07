package modeldesign

import "github.com/mathieu-sabatier/twin-model/internal/dsl"

// buildPerspectiveNodes emits FolderType grouping objects for every export:true
// perspective. Each node organizes its child nodes and member instances via
// forward Organizes; root nodes (not a child of any node in the perspective) are
// anchored under ObjectsFolder via an inverse Organizes. Deterministic: iterates
// the AST's ordered slices.
func buildPerspectiveNodes(m *dsl.Model) []any {
	var out []any
	for _, p := range m.Perspectives {
		if !p.Export {
			continue
		}
		childOf := map[string]bool{}
		for _, nd := range p.Nodes {
			for _, c := range nd.Children {
				childOf[c] = true
			}
		}
		for _, nd := range p.Nodes {
			sym := p.ID + "_" + nd.ID
			refs := []xmlReference{}
			if !childOf[nd.ID] {
				refs = append(refs, xmlReference{
					IsInverse: true, ReferenceType: "ua:Organizes", TargetID: "ua:ObjectsFolder",
				})
			}
			for _, c := range nd.Children {
				refs = append(refs, xmlReference{
					ReferenceType: "ua:Organizes", TargetID: p.ID + "_" + c,
				})
			}
			for _, mem := range nd.Members {
				refs = append(refs, xmlReference{
					ReferenceType: "ua:Organizes", TargetID: mem, // instance SymbolicName, unprefixed
				})
			}
			out = append(out, xmlInstance{
				SymbolicName:   sym,
				TypeDefinition: "ua:FolderType",
				References:     xmlReferences{References: refs},
			})
		}
	}
	return out
}
