// Low-level MySQL wire-protocol packet framing needed to terminate
// authentication at the gateway (see mysqlauth.go). Deliberately
// self-contained rather than sharing internal/proxy's private helpers —
// they serve a different purpose (query-content inspection on an already
// -established connection) and pulling them into a shared package isn't
// warranted for this much overlap; see the package doc in mysqlauth.go for
// why authentication itself needs to be terminated here at all.
package gateway

import (
	"fmt"
	"io"
)

// readWirePacket reads exactly one MySQL protocol packet from r: a 3-byte
// little-endian length, a 1-byte sequence number, then that many payload
// bytes. A logical message larger than 2^24-1 bytes would span multiple
// physical packets (continuation packets carry length 0xFFFFFF) — not
// handled here, since every packet this file reads or builds (handshakes,
// OK/ERR, auth-more-data, RSA public keys) is always far smaller than that.
func readWirePacket(r io.Reader) (seq byte, payload []byte, err error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	length := int(header[0]) | int(header[1])<<8 | int(header[2])<<16
	payload = make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
	}
	return header[3], payload, nil
}

// writeWirePacket frames payload as one MySQL protocol packet and writes it
// to w. Same 2^24-1 single-packet limitation as readWirePacket.
func writeWirePacket(w io.Writer, seq byte, payload []byte) error {
	if len(payload) >= 1<<24 {
		return fmt.Errorf("mysql 封包長度超過單一封包上限（%d bytes）", len(payload))
	}
	packet := make([]byte, 4+len(payload))
	packet[0] = byte(len(payload))
	packet[1] = byte(len(payload) >> 8)
	packet[2] = byte(len(payload) >> 16)
	packet[3] = seq
	copy(packet[4:], payload)
	_, err := w.Write(packet)
	return err
}

// isOKPacket reports whether payload is an OK packet (vs. ERR or something
// else) under CLIENT_PROTOCOL_41 (universal today): header byte 0x00, or
// 0xFE when short enough to be an EOF-as-OK per the deprecate-EOF rules —
// callers here never negotiate CLIENT_DEPRECATE_EOF, so only 0x00 actually
// occurs, but both are checked defensively.
func isOKPacket(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}
	if payload[0] == 0x00 {
		return true
	}
	return payload[0] == 0xfe && len(payload) < 9
}

// isErrPacket reports whether payload is an ERR packet.
func isErrPacket(payload []byte) bool {
	return len(payload) > 0 && payload[0] == 0xff
}

// parseErrMessage best-effort extracts the human-readable message from an
// ERR packet's payload (skips the leading 0xFF, the 2-byte error code, and —
// if present — the '#' + 5-byte SQLSTATE marker).
func parseErrMessage(payload []byte) string {
	i := 3
	if i < len(payload) && payload[i] == '#' {
		i += 6
	}
	if i > len(payload) {
		return ""
	}
	return string(payload[i:])
}

// buildOKPacket builds a minimal OK packet (CLIENT_PROTOCOL_41 shape):
// header 0x00, affected_rows=0, last_insert_id=0, status_flags=0,
// warnings=0.
func buildOKPacket() []byte {
	payload := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	return payload
}
