// Code generated by "stringer -type FileType -trimprefix FileType"; DO NOT EDIT.

package qualisys

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[FileTypeC3D-5]
	_ = x[FileTypeQTM-8]
}

const (
	_FileType_name_0 = "C3D"
	_FileType_name_1 = "QTM"
)

func (i FileType) String() string {
	switch {
	case i == 5:
		return _FileType_name_0
	case i == 8:
		return _FileType_name_1
	default:
		return "FileType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
