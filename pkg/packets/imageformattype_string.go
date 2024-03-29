// Code generated by "stringer -type ImageFormatType -trimprefix ImageFormatType"; DO NOT EDIT.

package packets

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ImageFormatTypeRawGreyscale-0]
	_ = x[ImageFormatTypeRawBGR-1]
	_ = x[ImageFormatTypeJPG-2]
	_ = x[ImageFormatTypePNG-3]
}

const _ImageFormatType_name = "RawGreyscaleRawBGRJPGPNG"

var _ImageFormatType_index = [...]uint8{0, 12, 18, 21, 24}

func (i ImageFormatType) String() string {
	if i >= ImageFormatType(len(_ImageFormatType_index)-1) {
		return "ImageFormatType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _ImageFormatType_name[_ImageFormatType_index[i]:_ImageFormatType_index[i+1]]
}
