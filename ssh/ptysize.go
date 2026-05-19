package ssh

import "encoding/binary"

// parsePtyReq extracts (cols, rows) from a "pty-req" payload per
// RFC 4254 §6.2:
//
//	string TERM environment variable value
//	uint32 terminal width, characters
//	uint32 terminal height, rows
//	uint32 terminal width, pixels
//	uint32 terminal height, pixels
//	string encoded terminal modes
//
// Returns (0, 0, false) if the payload is shorter than we need.
// Pixel dimensions and TERM string are discarded — readline / x/term
// only cares about character columns/rows.
func parsePtyReq(payload []byte) (cols, rows uint32, ok bool) {
	if len(payload) < 4 {
		return 0, 0, false
	}
	termLen := binary.BigEndian.Uint32(payload[:4])
	// 4 bytes length + termLen bytes string + 4*4 bytes (cols, rows, w_px, h_px)
	if uint32(len(payload)) < 4+termLen+16 {
		return 0, 0, false
	}
	rest := payload[4+termLen:]
	cols = binary.BigEndian.Uint32(rest[0:4])
	rows = binary.BigEndian.Uint32(rest[4:8])
	return cols, rows, true
}

// parseWindowChange extracts (cols, rows) from a "window-change"
// payload per RFC 4254 §6.7:
//
//	uint32 terminal width, columns
//	uint32 terminal height, rows
//	uint32 terminal width, pixels
//	uint32 terminal height, pixels
func parseWindowChange(payload []byte) (cols, rows uint32, ok bool) {
	if len(payload) < 16 {
		return 0, 0, false
	}
	cols = binary.BigEndian.Uint32(payload[0:4])
	rows = binary.BigEndian.Uint32(payload[4:8])
	return cols, rows, true
}
