package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	ProtocolVersion uint8  = 1
	HeaderSize             = 6 // 1 (Version) + 1 (Type) + 4 (Length)
	MaxPayloadSize  uint32 = 64 * 1024
)

type MsgType uint8

const (
	TypeHello   MsgType = 0x01
	TypeMessage MsgType = 0x02
	TypeError   MsgType = 0x05
)

var (
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	ErrPayloadTooLarge    = errors.New("payload exceeds max size")
)

type Packet struct {
	Version uint8
	Type    MsgType
	Payload []byte
}

// WritePacket сериализует пакет в бинарный формат и записывает его в w.
func WritePacket(w io.Writer, p Packet) error {
	if uint64(len(p.Payload)) > uint64(MaxPayloadSize) {
		return fmt.Errorf("%w: %d bytes", ErrPayloadTooLarge, len(p.Payload))
	}

	version := p.Version
	if version == 0 {
		version = ProtocolVersion
	} else if version != ProtocolVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedVersion, version)
	}

	buf := make([]byte, HeaderSize+len(p.Payload))
	buf[0] = version
	buf[1] = byte(p.Type)
	binary.BigEndian.PutUint32(buf[2:6], uint32(len(p.Payload)))
	copy(buf[HeaderSize:], p.Payload)

	_, err := w.Write(buf)
	if err != nil {
		return fmt.Errorf("write packet: %w", err)
	}
	return nil
}

// ReadPacket вычитывает один полный пакет из потока r.
// Если поток корректно закрыт между пакетами, возвращает io.EOF.
func ReadPacket(r io.Reader) (Packet, error) {
	var header [HeaderSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Packet{}, err
	}

	version := header[0]
	if version != ProtocolVersion {
		return Packet{}, fmt.Errorf("%w: %d", ErrUnsupportedVersion, version)
	}

	msgType := MsgType(header[1])
	length := binary.BigEndian.Uint32(header[2:6])

	if length > MaxPayloadSize {
		return Packet{}, fmt.Errorf("%w: %d bytes", ErrPayloadTooLarge, length)
	}

	if length == 0 {
		return Packet{
			Version: version,
			Type:    msgType,
		}, nil
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		if errors.Is(err, io.EOF) {
			return Packet{}, io.ErrUnexpectedEOF
		}
		return Packet{}, fmt.Errorf("read payload: %w", err)
	}

	return Packet{
		Version: version,
		Type:    msgType,
		Payload: payload,
	}, nil
}
