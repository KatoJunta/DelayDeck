package rtmpcompat

import (
	"io"

	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

type MetaForwarder interface {
	ForwardOnMetaData(payload []byte)
}

var metaForwarder MetaForwarder

func SetMetaForwarder(f MetaForwarder) {
	metaForwarder = f
}

func init() {
	rtmpmsg.DataBodyDecoders["onMetaData"] = decodeOnMetaData
}

func decodeOnMetaData(r io.Reader, _ rtmpmsg.AMFDecoder, v *rtmpmsg.AMFConvertible) error {
	payload, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if metaForwarder != nil && len(payload) > 0 {
		metaForwarder.ForwardOnMetaData(append([]byte(nil), payload...))
	}

	*v = &rtmpmsg.NetStreamSetDataFrame{}
	return nil
}
