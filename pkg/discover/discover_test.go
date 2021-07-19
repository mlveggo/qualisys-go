package discover_test

import (
	"encoding/binary"
	"testing"

	"github.com/mlveggo/qualisys-go/pkg/discover"
	"gotest.tools/assert"
)

func Test_UnmarshalBinaryEmptyBuffer(t *testing.T) {
	var dr discover.Response
	b := make([]byte, 10)
	err := dr.UnmarshalBinary(b)
	assert.ErrorContains(t, err, "too little data to UnmarshalBinary from")
}

func Test_UnmarshalBinaryGoodBuffer(t *testing.T) {
	b := make([]byte, 60)
	binary.LittleEndian.PutUint32(b, 60)
	binary.LittleEndian.PutUint32(b[4:8], 1)
	copy(b[8:48], "TestHost, QTM 2025.1 32300, 1234 cameras  ")
	binary.BigEndian.PutUint16(b[58:60], 22226)
	var dr discover.Response
	err := dr.UnmarshalBinary(b)
	assert.Assert(t, err == nil)
	assert.Equal(t, dr.Cameras, 1234)
	assert.Equal(t, dr.QtmVersion, "QTM 2025.1 32300")
	assert.Equal(t, dr.Hostname, "TestHost")
}
