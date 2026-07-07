package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCatalogList(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"catalog", "list"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d; stderr: %s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{"DI", "Machinery", "ISA95", "http://opcfoundation.org/UA/DI/"} {
		if !strings.Contains(s, want) {
			t.Errorf("catalog list missing %q:\n%s", want, s)
		}
	}
}

func TestCatalogShowDeviceType(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"catalog", "show", "DI:DeviceType"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d; stderr: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "DeviceType") {
		t.Errorf("show DI:DeviceType:\n%s", out.String())
	}
}
