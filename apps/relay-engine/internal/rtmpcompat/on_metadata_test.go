package rtmpcompat

import (
	"bytes"
	"testing"

	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

func TestOnMetaDataDecoderRegistersAndForwards(t *testing.T) {
	var forwarded []byte
	SetMetaForwarder(metaCapture{payload: &forwarded})
	defer SetMetaForwarder(nil)

	decoder, ok := rtmpmsg.DataBodyDecoders["onMetaData"]
	if !ok {
		t.Fatal("onMetaData decoder not registered")
	}

	var value rtmpmsg.AMFConvertible
	if err := decoder(bytes.NewReader([]byte{0x08, 0x00, 0x00, 0x00, 0x01}), nil, &value); err != nil {
		t.Fatalf("decode onMetaData: %v", err)
	}
	if len(forwarded) == 0 {
		t.Fatal("expected metadata payload to be forwarded")
	}
}

type metaCapture struct {
	payload *[]byte
}

func (m metaCapture) ForwardOnMetaData(payload []byte) {
	*m.payload = append((*m.payload)[:0], payload...)
}
