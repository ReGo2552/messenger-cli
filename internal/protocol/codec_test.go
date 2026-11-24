package protocol

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

func TestPacketStream(t *testing.T) {
	var stream bytes.Buffer
	for _, typ := range []MsgType{TypeHello, TypeMessage} {
		if err := WritePacket(&stream, Packet{Type: typ, Payload: []byte("{}")}); err != nil {
			t.Fatal(err)
		}
	}
	want := []byte{1, 1, 0, 0, 0, 2, '{', '}', 1, 2, 0, 0, 0, 2, '{', '}'}
	if !bytes.Equal(stream.Bytes(), want) {
		t.Fatalf("encoded % x, want % x", stream.Bytes(), want)
	}
	reader := iotest.OneByteReader(&stream)
	for _, typ := range []MsgType{TypeHello, TypeMessage} {
		p, err := ReadPacket(reader)
		if err != nil || p.Version != 1 || p.Type != typ || string(p.Payload) != "{}" {
			t.Fatalf("read type %d: packet %+v, error %v", typ, p, err)
		}
	}
	if _, err := ReadPacket(reader); err != io.EOF {
		t.Fatalf("end of stream: got %v, want EOF", err)
	}
}

func TestInvalidFrame(t *testing.T) {
	for _, tc := range []struct {
		header []byte
		want   error
	}{
		{[]byte{2, 2, 0, 0, 0, 0}, ErrUnsupportedVersion},
		{[]byte{1, 2, 0, 1, 0, 1}, ErrPayloadTooLarge},
		{[]byte{1, 2, 0, 0, 0, 1}, io.ErrUnexpectedEOF},
	} {
		if _, err := ReadPacket(bytes.NewReader(tc.header)); !errors.Is(err, tc.want) {
			t.Errorf("header % x: got %v, want %v", tc.header, err, tc.want)
		}
	}
	if err := WritePacket(io.Discard, Packet{Version: 2}); !errors.Is(err, ErrUnsupportedVersion) {
		t.Errorf("write version 2: got %v, want ErrUnsupportedVersion", err)
	}
}

func TestPayloadLimit(t *testing.T) {
	var stream bytes.Buffer
	p := Packet{Type: TypeMessage, Payload: make([]byte, 65536)}
	if err := WritePacket(&stream, p); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPacket(&stream)
	if err != nil || !bytes.Equal(got.Payload, p.Payload) {
		t.Fatalf("read maximum payload: length %d, error %v", len(got.Payload), err)
	}
	p.Payload = append(p.Payload, 0)
	if err := WritePacket(&stream, p); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("write oversized payload: got %v, want ErrPayloadTooLarge", err)
	}
}
