package nodeset

import (
	"fmt"
	"strconv"
	"strings"
)

// OpcUaCoreURI is ns0, the implicit namespace at file-local index 0.
const OpcUaCoreURI = "http://opcfoundation.org/UA/"

// NodeRef is a NodeId resolved to an absolute namespace URI + identifier.
type NodeRef struct {
	URI string
	ID  string // e.g. "i=1002"
}

// resolveNodeID maps a written NodeId ("i=58" or "ns=1;i=1002") to its absolute
// namespace. File-local index 0 is OPC UA core; index k>=1 is NamespaceURIs[k-1].
func (ns *NodeSet) resolveNodeID(raw string) (NodeRef, error) {
	raw = strings.TrimSpace(raw)
	idx := 0
	id := raw
	if rest, ok := strings.CutPrefix(raw, "ns="); ok {
		semi := strings.IndexByte(rest, ';')
		if semi < 0 {
			return NodeRef{}, fmt.Errorf("nodeset: malformed NodeId %q", raw)
		}
		n, err := strconv.Atoi(rest[:semi])
		if err != nil || n < 0 {
			return NodeRef{}, fmt.Errorf("nodeset: bad ns index in %q", raw)
		}
		idx = n
		id = rest[semi+1:]
	}
	if idx == 0 {
		return NodeRef{URI: OpcUaCoreURI, ID: id}, nil
	}
	if idx-1 >= len(ns.NamespaceURIs) {
		return NodeRef{}, fmt.Errorf("nodeset: ns index %d out of range (%d uris)", idx, len(ns.NamespaceURIs))
	}
	return NodeRef{URI: ns.NamespaceURIs[idx-1], ID: id}, nil
}

// resolveRefType returns a reference's canonical NodeId, following <Aliases>.
func (ns *NodeSet) resolveRefType(r Reference) string {
	for _, a := range ns.Aliases {
		if a.Name == r.Type {
			return a.Value
		}
	}
	return r.Type
}

// Well-known ns0 reference-type and modelling-rule NodeIds.
const (
	refHasSubtype        = "i=45"
	refHasProperty       = "i=46"
	refHasComponent      = "i=47"
	refHasTypeDefinition = "i=40"
	refHasModellingRule  = "i=37"
	ruleMandatory        = "i=78"
	ruleOptional         = "i=80"
	ruleMandatoryPlace   = "i=11508"
	ruleOptionalPlace    = "i=11509"
)
