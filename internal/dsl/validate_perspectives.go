package dsl

import "regexp"

var symbolicNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (v *validator) checkPerspectives() {
	instSet := map[string]bool{}
	for _, inst := range v.m.Instances {
		instSet[inst.Name] = true
	}
	for _, p := range v.m.Perspectives {
		base := "perspectives/" + p.ID
		if p.Membership != "" && p.Membership != "exclusive" && p.Membership != "overlapping" {
			v.err(CodeUnknownMembershipMode, base+"/membership", p.Pos,
				"unknown membership mode %q (want exclusive|overlapping)", p.Membership)
		}
		exclusive := p.Membership != "overlapping" // default is exclusive

		nodeSet := map[string]bool{}
		for _, nd := range p.Nodes {
			nodeSet[nd.ID] = true
		}

		memberSeenIn := map[string]string{} // instance id -> first node id it appears in
		for _, nd := range p.Nodes {
			nbase := base + "/nodes/" + nd.ID
			for _, child := range nd.Children {
				if !nodeSet[child] {
					v.err(CodeUnknownPerspectiveNode, nbase+"/children", nd.Pos,
						"node %q references unknown perspective node %q", nd.ID, child)
				}
			}
			for _, mem := range nd.Members {
				if !instSet[mem] {
					v.err(CodeDanglingMember, nbase+"/members/"+mem, nd.Pos,
						"member %q is not a declared instance", mem)
					continue
				}
				if exclusive {
					if prev, dup := memberSeenIn[mem]; dup {
						v.err(CodeExclusiveMembership, nbase+"/members/"+mem, nd.Pos,
							"instance %q appears in nodes %q and %q of exclusive perspective %q",
							mem, prev, nd.ID, p.ID)
					} else {
						memberSeenIn[mem] = nd.ID
					}
				}
			}
		}
		v.checkPerspectiveCycles(p, nodeSet)
		if p.Export {
			v.checkPerspectiveExportable(p)
		}
	}
}

// checkPerspectiveCycles walks each node's children chain (DFS) reporting the
// first back-edge as a cycle. Mirrors checkInstanceCycles.
func (v *validator) checkPerspectiveCycles(p *Perspective, nodeSet map[string]bool) {
	byID := map[string]*PerspectiveNode{}
	for _, nd := range p.Nodes {
		byID[nd.ID] = nd
	}
	color := map[string]int{} // 0 unseen, 1 in-stack, 2 done
	var walk func(id string) bool
	walk = func(id string) bool {
		color[id] = 1
		for _, child := range byID[id].Children {
			if !nodeSet[child] {
				continue // dangling child already reported
			}
			switch color[child] {
			case 1:
				return true
			case 0:
				if walk(child) {
					return true
				}
			}
		}
		color[id] = 2
		return false
	}
	for _, nd := range p.Nodes {
		if color[nd.ID] == 0 && walk(nd.ID) {
			v.err(CodePerspectiveNodeCycle, "perspectives/"+p.ID+"/nodes/"+nd.ID, nd.Pos,
				"perspective %q has a node cycle through %q", p.ID, nd.ID)
			return // one cycle report per perspective is enough
		}
	}
}

func (v *validator) checkPerspectiveExportable(p *Perspective) {
	if !symbolicNameRe.MatchString(p.ID) {
		v.err(CodePerspectiveIDNotExportable, "perspectives/"+p.ID, p.Pos,
			"exported perspective id %q is not a valid SymbolicName ([A-Za-z_][A-Za-z0-9_]*)", p.ID)
	}
	for _, nd := range p.Nodes {
		if !symbolicNameRe.MatchString(nd.ID) {
			v.err(CodePerspectiveIDNotExportable, "perspectives/"+p.ID+"/nodes/"+nd.ID, nd.Pos,
				"exported perspective node id %q is not a valid SymbolicName", nd.ID)
		}
	}
}
