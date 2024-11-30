package portergosdk

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
)

// TODO
var ErrNotSuccess = errors.New("failed connection")

func parseReasonCode(code byte) string {
	switch code {
	case 0x81:
		return "malformed_packet"
	case 0x82:
		return "protocol_error"
	case 0x83:
		return "implementation_error"
	case 0x84:
		return "unsupported_protocol_version"
	case 0x85:
		return "invalid_client_id"
	case 0x86:
		return "bad_creadentials"
	case 0x87:
		return "not_authorized"
	case 0x88:
		return "server_unavailable"
	case 0x89:
		return "server_busy"
	case 0x8a:
		return "banned"
	case 0x8c:
		return "bad_authentication_method"
	case 0x90:
		return "invalid_topic"
	case 0x95:
		return "packet_too_large"
	case 0x97:
		return "quota_exceeded"
	case 0x99:
		return "payload_format_invalid"
	case 0x9a:
		return "retain_not_supported"
	case 0x9b:
		return "qos_not_supported"
	case 0x9c:
		return "user_another_server"
	case 0x9d:
		return "server_moved"
	case 0x9f:
		return "connection_rate_exceeded"
	default:
		return "unspecified_error"
	}
}

func buildConnect(
	cid string,
	keepAlive uint16,
	creds *credential,
	qos QoS,
	sessionExpiry uint32,
) ([]byte, error) {
	// make connect packet
	var (
		msg,
		vhBuff,
		lenBuff,
		idBuff,
		usrBuff,
		pwdBuff bytes.Buffer
	)

	if err := msg.WriteByte(ConnectCMD); err != nil {
		return nil, err
	}

	n, err := vhBuff.Write([]byte{
		0x00, 0x04, 0x4d, 0x51, 0x54, 0x54, 0x05,
	})
	if err != nil {
		return nil, err
	}

	props := make([]Prop, 0, 9)

	var flag uint8 = 0

	if qos > 0 {
		flag ^= uint8(qos & QoSFlag)
	}

	if sessionExpiry > 0 {
		se, err := NewProperty(
			Uint32,
			0x11,
			sessionExpiry,
		)
		if err != nil {
			return nil, err
		}
		props = append(props, se)
	}

	if creds != nil {
		authProp, err := NewProperty(
			EncString,
			0x15,
			creds.authMethod,
		)
		if err != nil {
			return nil, err
		}
		props = append(props, authProp)

		if creds.usr != nil {
			flag ^= 0x80
			if err := writeUTFString(&usrBuff, *creds.usr); err != nil {
				return nil, err
			}
			n += usrBuff.Len()
		}

		if creds.pwd != nil {
			flag ^= 0x40
			if err := writeUTFString(&pwdBuff, *creds.pwd); err != nil {
				return nil, err
			}
			n += pwdBuff.Len()
		}
	}

	// Var header flag
	if err := vhBuff.WriteByte(flag); err != nil {
		return nil, err
	}
	n++

	if err := writeUint16(&vhBuff, keepAlive); err != nil {
		return nil, err
	}
	n += 2

	var propLenBuff bytes.Buffer
	var propBuff bytes.Buffer

	for _, prop := range props {
		if err := propBuff.WriteByte(prop.key); err != nil {
			return nil, err
		}
		n++

		nv, err := propBuff.Write(prop.value)
		if err != nil {
			return nil, err
		}
		n += nv
	}

	if err := encodeVarInt(&propLenBuff, propBuff.Len()); err != nil {
		return nil, err
	}

	n += propBuff.Len()

	// id
	if err := writeUTFString(&idBuff, cid); err != nil {
		return nil, err
	}
	n += idBuff.Len()

	// usr & pwd
	if ul := usrBuff.Len(); ul > 0 {
		n += ul
	}

	if pl := pwdBuff.Len(); pl > 0 {
		n += pl
	}

	// Encode packet remaining length
	if err := encodeVarInt(&lenBuff, n); err != nil {
		return nil, err
	}

	whole := slices.Concat(
		lenBuff.Bytes(),
		vhBuff.Bytes(),
		propLenBuff.Bytes(),
		propBuff.Bytes(),
		idBuff.Bytes(),
		usrBuff.Bytes(),
		pwdBuff.Bytes(),
	)

	if _, err := msg.Write(whole); err != nil {
		return nil, err
	}

	return msg.Bytes(), nil
}

type connackResponse struct {
	code            byte
	description     string
	reason          string
	assignedID      string
	serverExpiry    uint16
	serverKeepAlive uint16
}

func readConnack(b []byte) (connackResponse, error) {
	cursor := 1
	length, err := decodeVarint(b[cursor:])
	if err != nil {
		return connackResponse{description: "failed to read packet"}, err
	}
	cursor += evalBytes(length)

	// Session flag
	_ = b[cursor]
	cursor++

	// Reason Code
	code := b[cursor]
	cr := connackResponse{
		code:        code,
		description: parseReasonCode(code),
	}
	cursor++

	// Properties
	propsLen, err := decodeVarint(b[cursor:])
	if err != nil {
		return cr, err
	}
	cursor += evalBytes(propsLen)

	ceil := cursor + (int(propsLen) - 1)
	for cursor < ceil {
		if cursor > int(length) {
			return cr, fmt.Errorf("malformed packet : cursor exceeded length")
		}

		switch b[cursor] {
		// Session Expiry
		case 0x11:
			cursor++
			exp, err := readIncrementUint16(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
			cr.serverExpiry = exp
			// Receive Maximum
		case 0x21:
			cursor++
			_, err := readIncrementUint16(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
		// Max QOS
		case 0x24:
			cursor++
			_ = readIncrementByte(b[cursor:], &cursor)
		// Max Packet Size
		case 0x27:
			cursor++
			if _, err := readIncrementUint32(b[cursor:], &cursor); err != nil {
				return cr, err
			}
		// Assigned Client ID
		case 0x12:
			cursor++
			id, err := readStringIncrement(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
			cr.assignedID = id
		// Reason
		case 0x1f:
			cursor++
			reason, err := readStringIncrement(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
			cr.reason = reason
			// Server Keep Alive
		case 0x13:
			cursor++
			ka, err := readIncrementUint16(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
			cr.serverKeepAlive = ka
			// Authentication Method
		case 0x15:
			cursor++
			if _, err := readStringIncrement(b[cursor:], &cursor); err != nil {
				return cr, err
			}
		default:
			cursor++
			continue
		}
	}

	return cr, nil
}
