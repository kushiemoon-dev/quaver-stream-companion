package testutil

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
)

// EncodeBreakdown builds a JudgementBreakdown value from a digit string such as "001235".
func EncodeBreakdown(digits string) string {
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	_, _ = zw.Write([]byte(digits))
	_ = zw.Close()
	raw := binary.LittleEndian.AppendUint32(nil, uint32(len(digits)))
	return base64.StdEncoding.EncodeToString(append(raw, zipped.Bytes()...))
}
