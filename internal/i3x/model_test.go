package i3x

import "testing"

func TestBuiltinJSONType(t *testing.T) {
	tests := []struct {
		name       string
		wantType   string
		wantFormat string
		wantOK     bool
	}{
		{"String", "string", "", true},
		{"Double", "number", "", true},
		{"Float", "number", "", true},
		{"Boolean", "boolean", "", true},
		{"SByte", "integer", "", true},
		{"Byte", "integer", "", true},
		{"Int16", "integer", "", true},
		{"UInt16", "integer", "", true},
		{"Int32", "integer", "", true},
		{"UInt32", "integer", "", true},
		{"Int64", "integer", "", true},
		{"UInt64", "integer", "", true},
		{"DateTime", "string", "date-time", true},
		{"NodeId", "", "", false},
		{"EquipmentState", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, format, ok := builtinJSONType(tt.name)
			if typ != tt.wantType || format != tt.wantFormat || ok != tt.wantOK {
				t.Errorf("builtinJSONType(%q) = (%q,%q,%v), want (%q,%q,%v)",
					tt.name, typ, format, ok, tt.wantType, tt.wantFormat, tt.wantOK)
			}
		})
	}
}

func TestWellKnownElementIDs(t *testing.T) {
	cases := map[string]string{
		elemBaseObjectType: "nsu=http://opcfoundation.org/UA/;s=BaseObjectType",
		elemFolderType:     "nsu=http://opcfoundation.org/UA/;s=FolderType",
		elemObjectsFolder:  "nsu=http://opcfoundation.org/UA/;s=ObjectsFolder",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}
