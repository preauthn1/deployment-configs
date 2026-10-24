package gatewayproto

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	want := ClientAuth{Version: Version, IDToken: [REDACTED]", Target: "baremetal-vplus33", ClientVersion: "dev"}
	var wire bytes.Buffer
	if err := WriteFrame(&wire, want); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}

	var got ClientAuth
	if err := ReadFrame(&wire, &got); err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if got != want {
		t.Fatalf("ReadFrame() = %#v, want %#v", got, want)
	}
}

func TestReadFrameRejectsOversizedPayloadBeforeAllocation(t *testing.T) {
	var wire bytes.Buffer
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], MaxFrameSize+1)
	wire.Write(header[:])

	var got ClientAuth
	err := ReadFrame(&wire, &got)
	if err == nil || !strings.Contains(err.Error(), "超出限制") {
		t.Fatalf("ReadFrame() error = %v, want size limit error", err)
	}
}

func TestReadFrameRejectsUnknownFields(t *testing.T) {
	payload := []byte(`{"version":1,"idToken":"token","target":"db","clientVersion":"dev","host":"0.0.0.0:3306"}`)
	var wire bytes.Buffer
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	wire.Write(header[:])
	wire.Write(payload)

	var got ClientAuth
	err := ReadFrame(&wire, &got)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("ReadFrame() error = %v, want unknown field error", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }

func TestWriteFrameRejectsShortWriter(t *testing.T) {
	err := WriteFrame(shortWriter{}, AuthResult{OK: true})
	if err == nil {
		t.Fatal("WriteFrame() error = nil, want short write error")
	}
}
