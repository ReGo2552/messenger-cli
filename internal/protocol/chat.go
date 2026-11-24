package protocol

import (
	"encoding/json"
	"io"
)

type Hello struct {
	Nick string `json:"nick"`
}

type Message struct {
	From string `json:"from,omitempty"`
	Text string `json:"text"`
}

type ErrorMessage struct {
	Message string `json:"message"`
}

func WriteJSON(w io.Writer, typ MsgType, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return WritePacket(w, Packet{Type: typ, Payload: payload})
}
