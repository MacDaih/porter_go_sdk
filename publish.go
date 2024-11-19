package portergosdk

import (
	"bytes"
	"fmt"
)

type ContentType string

const (
	Json = "json"
	Text = "text"
)

type AppMessage struct {
	MessageQoS  QoS
	TopicName   string
	Format      bool
	ContentType ContentType
	Correlation string
	SubID       string
	Payload     []byte
}

func buildPublish(appMsg AppMessage) ([]byte, error) {
	var header bytes.Buffer
	header.WriteByte(PublishCMD ^ (0 << 1))

	props := make([]Prop, 0, 7)
	propLen := 0

	if appMsg.Format {
		prop, err := NewProperty(Byte, 0x01, 1)
		if err != nil {
			return nil, err
		}
		props = append(props, prop)
		propLen += len(prop.value) + 1
	}

	if appMsg.ContentType != "" {
		prop, err := NewProperty(EncString, 0x03, appMsg.ContentType)
		if err != nil {
			return nil, err
		}
		props = append(props, prop)
		propLen += len(prop.value) + 1
	}

	var msg bytes.Buffer

	if err := writeUTFString(&msg, appMsg.TopicName); err != nil {
		return nil, err
	}

	if err := encodeVarInt(&msg, propLen); err != nil {
		return nil, err
	}
	for _, p := range props {
		msg.WriteByte(p.key)
		if _, err := msg.Write(p.value); err != nil {
			return nil, err
		}
	}

	np, err := msg.Write(appMsg.Payload)
	if err != nil {
		return nil, err
	}

	remLen := (len(appMsg.TopicName) + 2) + evalBytes(uint32(propLen)) + propLen + np
	if err := encodeVarInt(&header, remLen); err != nil {
		return nil, err

	}

	if _, err := header.Write(msg.Bytes()); err != nil {
		return nil, err
	}
	return header.Bytes(), nil
}

func readPublish(b []byte) (AppMessage, error) {
	var msg AppMessage
	cursor := 1
	// TODO publis cmd with retain, dup and qos
	// Remaining length
	length, err := decodeVarint(b[1:])
	if err != nil {
		return msg, err
	}

	if len(b) < int(length) {
		return msg, fmt.Errorf("malformed packet : invalid length")
	}

	cursor += evalBytes(length)
	// Read topic
	topic, err := readUTFString(b[cursor:])
	if err != nil {
		return msg, err
	}
	msg.TopicName = topic

	cursor += len(topic) + 2
	// No packet ID for now

	//read props
	propsLen, err := decodeVarint(b[cursor:])
	if err != nil {
		return msg, err
	}

	ceil := cursor + int(propsLen)
	for cursor < ceil {
		if cursor > int(length) {
			return msg, fmt.Errorf("malformed packet : cursor exceeded length")
		}
		switch b[cursor] {
		case 0x01:
			cursor++
			// payload format indicator
			indicator := b[cursor]
			msg.Format = indicator >= 1
			cursor++
		case 0x03:
			cursor++
			// content type
			content, err := readUTFString(b[cursor:])
			if err != nil {
				return msg, err
			}
			msg.ContentType = ContentType(content)
		default:
			cursor++
			continue
		}
	}

	if msg.Format {
		payload, err := readUTFString(b[cursor+1:])
		//msg.Payload = b[cursor+1 : length-1]
		if err != nil {
			return msg, err
		}
		msg.Payload = []byte(payload)
	}

	return msg, nil
}
