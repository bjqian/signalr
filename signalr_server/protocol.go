package signalr_server

import (
	"encoding/json"
	"log"
)

const recordSeparator = '\x1E'

var pingMsgBytes, _ = json.Marshal(PingMsg{Type: 6})

func verifyRecordSeparator(bytes []byte) {
	if bytes[len(bytes)-1] != recordSeparator {
		log.Fatalln("record separator not found")
	}
}

func appendRecordSeparator(bytes []byte) []byte {
	return append(bytes, recordSeparator)
}
