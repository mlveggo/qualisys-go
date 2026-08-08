package discover_test

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/mlveggo/qualisys-go/pkg/discover"
)

// These tests use only the standard library. The previous version depended on
// gotest.tools, an unmaintained v2 module pulled in solely for three assertion
// helpers; dropping it means `go test ./...` works without any module fetch.

func TestUnmarshalBinaryRejectsShortBuffer(t *testing.T) {
	var dr discover.Response
	err := dr.UnmarshalBinary(make([]byte, 10))
	if err == nil {
		t.Fatal("expected an error for a buffer below the minimum length")
	}
	if !strings.Contains(err.Error(), "too little data") {
		t.Errorf("got %v", err)
	}
}

func TestUnmarshalBinaryParsesResponse(t *testing.T) {
	b := make([]byte, 60)
	binary.LittleEndian.PutUint32(b, 60)
	binary.LittleEndian.PutUint32(b[4:8], 1)
	copy(b[8:48], "TestHost, QTM 2025.1 32300, 1234 cameras  ")
	binary.BigEndian.PutUint16(b[58:60], 22226)

	var dr discover.Response
	if err := dr.UnmarshalBinary(b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dr.Hostname != "TestHost" {
		t.Errorf("hostname = %q, want TestHost", dr.Hostname)
	}
	if dr.QtmVersion != "QTM 2025.1 32300" {
		t.Errorf("version = %q", dr.QtmVersion)
	}
	if dr.Cameras != 1234 {
		t.Errorf("cameras = %d, want 1234", dr.Cameras)
	}
	if dr.BasePort != 22226 {
		t.Errorf("baseport = %d, want 22226", dr.BasePort)
	}
}

func TestUnmarshalBinaryRejectsMalformedInfo(t *testing.T) {
	b := make([]byte, 60)
	binary.LittleEndian.PutUint32(b, 60)
	binary.LittleEndian.PutUint32(b[4:8], 1)
	copy(b[8:48], "no commas here at all")
	binary.BigEndian.PutUint16(b[58:60], 22226)

	var dr discover.Response
	if err := dr.UnmarshalBinary(b); err == nil {
		t.Error("expected an error for a malformed information field")
	}
}
