package portergosdk

import (
	"bytes"
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

	if appMsg.MessageQoS > 0 {
		if err := writeUint16(&msg, 0); err != nil {
			return nil, err
		}
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

func readPublish(pkt *packet) (AppMessage, error) {
	var msg AppMessage

	// Read topic
	topic, err := pkt.readString()
	if err != nil {
		return msg, err
	}
	msg.TopicName = topic

	// qos
	if (pkt.flags & 0x06) > 0 {
		if _, err := pkt.readUint16();err != nil {
			return msg, err
		}
	}

	if _, err := pkt.readProperties(8); err != nil {
		return msg, err
	}

	msg.Payload = pkt.buffer.Bytes()
	return msg, nil
}
