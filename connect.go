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
		return "Malformed Packet"
	case 0x82:
		return "Protocol Error"
	case 0x83:
		return "Implementation Error"
	case 0x84:
		return "Unsupported Protocol Version"
	case 0x85:
		return "Invalid Client ID"
	case 0x86:
		return "Bad Creadentials"
	case 0x87:
		return "Not Authorized"
	case 0x88:
		return "Server Unavailable"
	case 0x89:
		return "Server Busy"
	case 0x8a:
		return "Banned"
	case 0x8c:
		return "Bad Authentication Method"
	case 0x90:
		return "Invalid Topic"
	case 0x95:
		return "Packet Too Large"
	case 0x97:
		return "Quota Exceeded"
	case 0x99:
		return "Payload Format Invalid"
	case 0x9a:
		return "Retain Not Supported"
	case 0x9b:
		return "QoS Not Supported"
	case 0x9c:
		return "User Another Server"
	case 0x9d:
		return "Server Moved"
	case 0x9f:
		return "Connection Rate Exceeded"
	default:
		return "Unspecified Error"
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
	length, err := decodeVarint(b[1:])
	if err != nil {
		return connackResponse{description: "failed to read packet"}, err
	}

	if len(b) < int(length) {
		return connackResponse{description: "failed to read packet"}, fmt.Errorf("malformed packet : invalid length")
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

	ceil := cursor + int(propsLen)

	for cursor < ceil {
		//if cursor > int(length) {
		//	return cr, fmt.Errorf("malformed packet : cursor exceeded length")
		//}

		switch b[cursor] {
		case 0x11: // Session Expiry
			cursor++
			exp, err := readIncrementUint16(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
			cr.serverExpiry = exp
		case 0x21: // Receive Maximum
			cursor++
			_, err := readIncrementUint16(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
		case 0x24: // Max QOS
			cursor++
			_ = readIncrementByte(b[cursor:], &cursor)
		case 0x27: // Max Packet Size
			cursor++
			if _, err := readIncrementUint32(b[cursor:], &cursor); err != nil {
				return cr, err
			}
		case 0x12: // Assigned Client ID
			cursor++
			id, err := readStringIncrement(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
			cr.assignedID = id
		case 0x1f: // Reason
			cursor++
			reason, err := readStringIncrement(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
			cr.reason = reason
		case 0x13: // Server Keep Alive
			cursor++
			ka, err := readIncrementUint16(b[cursor:], &cursor)
			if err != nil {
				return cr, err
			}
			cr.serverKeepAlive = ka
		case 0x15: // Authentication Method
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
