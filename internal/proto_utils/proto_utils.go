package proto_utils

import (
	"encoding/binary"
	"google.golang.org/protobuf/proto"
	"io"
)

// Write a protobuf message to a writer with length prefixing.
func Write(w io.Writer, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(data)))
	combined := make([]byte, 0, len(buf)+len(data))
	combined = append(combined, buf...)
	combined = append(combined, data...)
	if _, err := w.Write(combined); err != nil {
		return err
	}
	return nil
}

// Read a protobuf message from a reader with length prefixing.
func Read(r io.Reader, msg proto.Message) error {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	size := binary.LittleEndian.Uint32(buf)
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return proto.Unmarshal(data, msg)
}
