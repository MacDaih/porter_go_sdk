package portergosdk

import (
	"bytes"
	"fmt"
)

type ContentType string

const (
	Json ContentType = "application/json"
	Text ContentType = "text/plain"
)

type AppMessage struct {
	MessageQoS  QoS
	TopicName   string
	Format      bool
	Content     ContentType
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
		prop, err := NewProperty(Byte, 0x01, byte(0x01))
		if err != nil {
			return nil, err
		}
		props = append(props, prop)
		propLen += len(prop.value) + 1
	}

	if appMsg.Content != "" {
		prop, err := NewProperty(EncString, 0x03, string(appMsg.Content))
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

	if err := writeUint16(&msg, 0); err != nil {
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

	var (
		payload bytes.Buffer
		np      int
		perr    error
	)
	if appMsg.Format {
		if err := writeUTFString(&payload, string(appMsg.Payload)); err != nil {
			return nil, err
		}

		np, perr = msg.Write(payload.Bytes())
		if perr != nil {
			return nil, perr
		}
	} else {
		np, perr = msg.Write(appMsg.Payload)
		if perr != nil {
			return nil, perr
		}
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
	// TODO publish cmd with retain, dup and qos

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

	fmt.Println("msg topic ", topic)
	cursor += len(topic) + 2

	// TODO No packet ID for now
	if _, err := readUint16(b[cursor:]); err != nil {
		return msg, err
	}
	cursor += 2

	//read props
	propsLen, err := decodeVarint(b[cursor:])
	if err != nil {
		return msg, err
	}
	cursor += evalBytes(propsLen)

	ceil := cursor + int(propsLen)

	fmt.Println(b[cursor:])
	for cursor < ceil {
		if cursor > int(length) {
			return msg, fmt.Errorf("malformed packet : cursor exceeded length")
		}
		switch b[cursor] {
		case 0x01:
			cursor++
			// payload format indicator

			fmt.Printf("msg format %d\n", b[cursor])
			msg.Format = readIncrementByte(b[cursor:], &cursor) >= 1
		case 0x03:
			cursor++
			// content type

			fmt.Printf("next 0x03 : %d %d", b[cursor], b[cursor+1])
			content, err := readStringIncrement(b[cursor:], &cursor)
			if err != nil {
				return msg, err
			}

			fmt.Printf("msg content %s\n", content)
			msg.Content = ContentType(content)
		default:

			fmt.Printf("what ?! %d", b[cursor])
			cursor++
			continue
		}
	}

	raw := b[cursor:]

	if msg.Format {
		payload, err := readUTFString(raw)
		if err != nil {
			return msg, err
		}
		msg.Payload = []byte(payload)
	} else {
		msg.Payload = raw
	}

	return msg, nil
}
