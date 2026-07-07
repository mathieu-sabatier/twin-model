package nodeset

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseMiniDI(t *testing.T) {
	f, err := os.Open("testdata/mini.di.NodeSet2.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ns, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(ns.Models) != 1 || ns.Models[0].ModelURI != "http://opcfoundation.org/UA/DI/" {
		t.Fatalf("model header: %+v", ns.Models)
	}
	if ns.Models[0].Version != "1.04.0" {
		t.Errorf("version = %q", ns.Models[0].Version)
	}
	if len(ns.Models[0].RequiredModels) != 1 {
		t.Fatalf("required models = %d", len(ns.Models[0].RequiredModels))
	}
	if got := len(ns.NamespaceURIs); got != 1 {
		t.Fatalf("namespace uris = %d", got)
	}
	var dt *Node
	for i := range ns.Nodes {
		if ns.Nodes[i].BrowseName == "1:DeviceType" {
			dt = &ns.Nodes[i]
		}
	}
	if dt == nil {
		t.Fatal("DeviceType node not found")
	}
	if dt.Class != "ObjectType" || !dt.IsAbstract {
		t.Errorf("DeviceType class=%q abstract=%v", dt.Class, dt.IsAbstract)
	}
	if len(dt.Refs) != 2 {
		t.Errorf("DeviceType refs = %d", len(dt.Refs))
	}
}

func TestParseEnumEncodings(t *testing.T) {
	doc := func(inner string) string {
		return `<UANodeSet xmlns="http://opcfoundation.org/UA/2011/03/UANodeSet.xsd">` + inner + `</UANodeSet>`
	}
	tests := []struct {
		name     string
		xml      string
		browse   string
		wantDef  []EnumMember
		wantStr  []string
		wantVals []EnumMember
	}{
		{
			name: "definition enum fields",
			xml: doc(`<UADataType NodeId="ns=1;i=4871" BrowseName="1:Lvl">
				<Definition Name="Lvl">
					<Field Name="Enterprise" Value="0" />
					<Field Name="Site" Value="1" />
					<Field Name="Area" Value="2" />
				</Definition></UADataType>`),
			browse:  "1:Lvl",
			wantDef: []EnumMember{{"Enterprise", 0}, {"Site", 1}, {"Area", 2}},
		},
		{
			name: "struct definition is not an enum",
			xml: doc(`<UADataType NodeId="ns=1;i=9000" BrowseName="1:S">
				<Definition Name="S">
					<Field Name="FieldA" DataType="String" />
					<Field Name="FieldB" DataType="Int32" />
				</Definition></UADataType>`),
			browse:  "1:S",
			wantDef: nil,
		},
		{
			name: "enum strings implicit indices",
			xml: doc(`<UAVariable NodeId="ns=1;i=4872" BrowseName="EnumStrings" DataType="LocalizedText">
				<Value><ListOfLocalizedText xmlns="http://opcfoundation.org/UA/2008/02/Types.xsd">
					<LocalizedText><Text>Enterprise</Text></LocalizedText>
					<LocalizedText><Text>Site</Text></LocalizedText>
					<LocalizedText><Text>Area</Text></LocalizedText>
				</ListOfLocalizedText></Value></UAVariable>`),
			browse:  "EnumStrings",
			wantStr: []string{"Enterprise", "Site", "Area"},
		},
		{
			name: "enum values extension objects",
			xml: doc(`<UAVariable NodeId="ns=1;i=6001" BrowseName="EnumValues" DataType="EnumValueType">
				<Value><uax:ListOfExtensionObject xmlns:uax="http://opcfoundation.org/UA/2008/02/Types.xsd">
					<uax:ExtensionObject><uax:Body><uax:EnumValueType>
						<uax:Value>0</uax:Value><uax:DisplayName><uax:Text>Dimmed</uax:Text></uax:DisplayName>
					</uax:EnumValueType></uax:Body></uax:ExtensionObject>
					<uax:ExtensionObject><uax:Body><uax:EnumValueType>
						<uax:Value>1</uax:Value><uax:DisplayName><uax:Text>Blinking</uax:Text></uax:DisplayName>
					</uax:EnumValueType></uax:Body></uax:ExtensionObject>
				</uax:ListOfExtensionObject></Value></UAVariable>`),
			browse:   "EnumValues",
			wantVals: []EnumMember{{"Dimmed", 0}, {"Blinking", 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, err := Parse(strings.NewReader(tt.xml))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			var n *Node
			for i := range ns.Nodes {
				if ns.Nodes[i].BrowseName == tt.browse {
					n = &ns.Nodes[i]
				}
			}
			if n == nil {
				t.Fatalf("node %q not found", tt.browse)
			}
			if !reflect.DeepEqual(n.EnumMembers, tt.wantDef) {
				t.Errorf("EnumMembers = %+v, want %+v", n.EnumMembers, tt.wantDef)
			}
			if !reflect.DeepEqual(n.EnumStrings, tt.wantStr) {
				t.Errorf("EnumStrings = %+v, want %+v", n.EnumStrings, tt.wantStr)
			}
			if !reflect.DeepEqual(n.EnumValues, tt.wantVals) {
				t.Errorf("EnumValues = %+v, want %+v", n.EnumValues, tt.wantVals)
			}
		})
	}
}
