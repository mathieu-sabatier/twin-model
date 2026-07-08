package modeldesign

import (
	"strings"
	"testing"
)

func TestEmitOrgSpineDeterministic(t *testing.T) {
	src := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-06 }\n" +
		"imports: { OpcUa: http://opcfoundation.org/UA/, ISA95: http://www.OPCFoundation.org/UA/2013/01/ISA95 }\n" +
		"instances:\n" +
		"  Acme: { type: ISA95:EquipmentType, level: Enterprise, under: OpcUa:ObjectsFolder }\n" +
		"  Site1: { type: ISA95:EquipmentType, level: Site, under: Acme }\n"
	a := emitWithCatalog(t, src)
	b := emitWithCatalog(t, src)
	if a != b {
		t.Errorf("emit not byte-identical across runs")
	}
	if !strings.Contains(a, `SymbolicName="Site1"`) || !strings.Contains(a, `<opc:TargetId>Acme</opc:TargetId>`) {
		t.Errorf("org node/Organizes edge missing:\n%s", a)
	}
}
