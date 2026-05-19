package ssh

import (
	"encoding/binary"
	"testing"
)

// makePtyReqPayload constructs a well-formed pty-req payload for
// testing. Fields per RFC 4254:
//
//	string TERM, uint32 cols, uint32 rows, uint32 wpx, uint32 hpx, string modes
func makePtyReqPayload(term string, cols, rows uint32) []byte {
	b := make([]byte, 0, 4+len(term)+16+4)
	// TERM string
	tlen := make([]byte, 4)
	binary.BigEndian.PutUint32(tlen, uint32(len(term)))
	b = append(b, tlen...)
	b = append(b, []byte(term)...)
	// 4 uint32s
	four := make([]byte, 16)
	binary.BigEndian.PutUint32(four[0:4], cols)
	binary.BigEndian.PutUint32(four[4:8], rows)
	binary.BigEndian.PutUint32(four[8:12], 0) // width pixels
	binary.BigEndian.PutUint32(four[12:16], 0) // height pixels
	b = append(b, four...)
	// Empty modes string
	b = append(b, 0, 0, 0, 0)
	return b
}

func makeWindowChangePayload(cols, rows uint32) []byte {
	b := make([]byte, 16)
	binary.BigEndian.PutUint32(b[0:4], cols)
	binary.BigEndian.PutUint32(b[4:8], rows)
	binary.BigEndian.PutUint32(b[8:12], 0)
	binary.BigEndian.PutUint32(b[12:16], 0)
	return b
}

func TestParsePtyReq(t *testing.T) {
	cases := []struct {
		name       string
		payload    []byte
		wantCols   uint32
		wantRows   uint32
		wantOk     bool
	}{
		{"normal", makePtyReqPayload("xterm-256color", 132, 50), 132, 50, true},
		{"empty TERM", makePtyReqPayload("", 80, 24), 80, 24, true},
		{"large", makePtyReqPayload("vt100", 250, 100), 250, 100, true},
		{"truncated", []byte{0, 0, 0, 0}, 0, 0, false},
		{"empty", []byte{}, 0, 0, false},
	}
	for _, tc := range cases {
		cols, rows, ok := parsePtyReq(tc.payload)
		if ok != tc.wantOk {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.wantOk)
			continue
		}
		if cols != tc.wantCols || rows != tc.wantRows {
			t.Errorf("%s: cols=%d rows=%d, want %d/%d",
				tc.name, cols, rows, tc.wantCols, tc.wantRows)
		}
	}
}

func TestParseWindowChange(t *testing.T) {
	cases := []struct {
		name     string
		payload  []byte
		wantCols uint32
		wantRows uint32
		wantOk   bool
	}{
		{"normal", makeWindowChangePayload(120, 40), 120, 40, true},
		{"min", makeWindowChangePayload(1, 1), 1, 1, true},
		{"truncated", []byte{0, 0, 0, 80}, 0, 0, false},
		{"empty", nil, 0, 0, false},
	}
	for _, tc := range cases {
		cols, rows, ok := parseWindowChange(tc.payload)
		if ok != tc.wantOk {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.wantOk)
			continue
		}
		if cols != tc.wantCols || rows != tc.wantRows {
			t.Errorf("%s: cols=%d rows=%d, want %d/%d",
				tc.name, cols, rows, tc.wantCols, tc.wantRows)
		}
	}
}
