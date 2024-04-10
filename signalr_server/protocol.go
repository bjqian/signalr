package signalr_server

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/vmihailenco/msgpack/v5"
	"log"
	"reflect"
)

type protocol interface {
	verifyAndRemoveMessageSeparator(bytes []byte) []byte
	appendMessageSeparator(bytes []byte) []byte
	pingMsg() []byte
	marshal(v any) ([]byte, error)
	unmarshal([]byte) (any, error)
	unmarshalArgument([]byte, reflect.Type) (reflect.Value, error)
}

type jsonProtocol struct {
}

type msgpackProtocol struct {
}

const recordSeparator = '\x1E'

var pingMsgBytes, _ = json.Marshal(PingMsg{Type: 6})

func (p *jsonProtocol) verifyAndRemoveMessageSeparator(bytes []byte) []byte {
	if bytes[len(bytes)-1] != recordSeparator {
		log.Fatalln("record separator not found")
	}
	return bytes[:len(bytes)-1]
}

func (p *jsonProtocol) appendMessageSeparator(bytes []byte) []byte {
	return append(bytes, recordSeparator)
}

func (p *jsonProtocol) pingMsg() []byte {
	return pingMsgBytes
}

func (p *jsonProtocol) marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (p *jsonProtocol) unmarshal(raw []byte) (any, error) {
	var baseType = BaseType{}
	err := json.Unmarshal(raw, &baseType)
	if err != nil {
		return nil, err
	}
	switch baseType.Type {
	case StreamInvocationType:
		fallthrough
	case InvocationType:
		var invocationRaw = InvocationWithJsonRawArguments{}
		err = json.Unmarshal(raw, &invocationRaw)
		if err != nil {
			return nil, err
		}
		invocation := Invocation{
			Type:         invocationRaw.Type,
			InvocationId: invocationRaw.InvocationId,
			Target:       invocationRaw.Target,
		}
		for _, arg := range invocationRaw.Arguments {
			invocation.Arguments = append(invocation.Arguments, []byte(arg))
		}
		return invocation, nil
	case PingType:
		return PingMsg{Type: PingType}, nil
	default:
		return nil, errors.New("unknown type")
	}
}

func (p *jsonProtocol) unmarshalArgument(raw []byte, t reflect.Type) (reflect.Value, error) {
	v := reflect.New(t)
	err := json.Unmarshal(raw, v.Interface())
	return v.Elem(), err
}

func (p *msgpackProtocol) verifyAndRemoveMessageSeparator(raw []byte) []byte {
	reader := bytes.NewReader(raw)
	decoder := msgpack.NewDecoder(reader)
	var number int
	if err := decoder.Decode(&number); err != nil {
		log.Fatal(err)
	}
	bytesRead := reader.Size() - int64(reader.Len())
	return raw[bytesRead:]
}

func (p *msgpackProtocol) appendMessageSeparator(raw []byte) []byte {
	prefix, err := msgpack.Marshal(len(raw))
	if err != nil {
		log.Fatal(err)
	}
	result := append(prefix, raw...)
	return result
}

var pingMsgBytesMsgpack = []byte{0x91, 0x06}

func (p *msgpackProtocol) pingMsg() []byte {
	return pingMsgBytesMsgpack
}

func (p *msgpackProtocol) marshal(v any) ([]byte, error) {
	switch m := v.(type) {
	case PingMsg:
		return pingMsgBytesMsgpack, nil
	case Invocation:
		var result = make([]any, 6)
		result[0] = m.Type
		result[1] = make(map[string]string)
		if m.InvocationId != "" {
			result[2] = m.InvocationId
		}
		result[3] = m.Target
		if len(m.Arguments) > 0 {
			result[4] = m.Arguments
		} else {
			result[4] = make([]any, 0)
		}
		result[5] = make([]string, 0)
		return msgpack.Marshal(result)
	case Completion:
		var result = make([]any, 5)
		result[0] = m.Type
		result[1] = make(map[string]string)
		if m.InvocationId != "" {
			result[2] = m.InvocationId
		}
		if m.Error != "" {
			result[3] = 1
			result[4] = m.Error
		} else {
			if m.Result == nil {
				result[3] = 2
				result = result[:5]
			} else {
				result[3] = 3
				result[4] = m.Result
			}
		}
		return msgpack.Marshal(result)
	default:
		return nil, errors.New("unknown type")
	}
}

func (p *msgpackProtocol) unmarshal(raw []byte) (any, error) {
	var base []msgpack.RawMessage
	err := msgpack.Unmarshal(raw, &base)

	if err != nil {
		return nil, err
	}
	var t int
	if len(base) == 0 {
		return nil, errors.New("empty message")
	}
	if err := msgpack.Unmarshal(base[0], &t); err != nil {
		return nil, err
	}
	switch t {
	case StreamInvocationType:
		fallthrough
	case InvocationType:
		if len(base) != 6 {
			return nil, errors.New("invalid invocation message")
		}
		var invocation = Invocation{
			Type: t,
		}
		var headers map[string]string
		if err := msgpack.Unmarshal(base[1], &headers); err != nil {
			return nil, err
		}
		if base[2] != nil {
			if err := msgpack.Unmarshal(base[2], &invocation.InvocationId); err != nil {
				return nil, err
			}
		}
		if err := msgpack.Unmarshal(base[3], &invocation.Target); err != nil {
			return nil, err
		}
		var arguments []msgpack.RawMessage
		if err := msgpack.Unmarshal(base[4], &arguments); err != nil {
			return nil, err
		}
		var streamIds []string
		if err := msgpack.Unmarshal(base[5], &streamIds); err != nil {
			return nil, err
		}
		for _, argument := range arguments {
			invocation.Arguments = append(invocation.Arguments, []byte(argument))
		}
		return invocation, nil
	case PingType:
		return PingMsg{Type: PingType}, nil
	default:
		return nil, errors.New("unknown type")
	}
}

func (p *msgpackProtocol) unmarshalArgument(raw []byte, t reflect.Type) (reflect.Value, error) {
	v := reflect.New(t)
	err := msgpack.Unmarshal(raw, v.Interface())
	return v.Elem(), err
}
