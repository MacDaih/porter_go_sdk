package portergosdk

import (
	"bytes"
	"errors"
)

type packetType byte

const (
	connectcmd     = 0x10
	connackcmd     = 0x20
	publishcmd     = 0x30
	pubackcmd      = 0x40
	pubreccmd      = 0x50
	pubrelcmd      = 0x60
	pubcompcmd     = 0x70
	subscribecmd   = 0x80
	subackcmd      = 0x90
	unsubscribecmd = 0xA0
	unsubackcmd    = 0xB0
	pingreqcmd     = 0xC0
	pingrespcmd    = 0xD0
	disconnectcmd  = 0xE0
	authcmd        = 0xF0
)

type packet struct {
	cmd    packetType
	flags  byte
	buffer *bytes.Buffer
	length int
}

var (
	ErrInvalidCommand error = errors.New("unrecognized packet command")
	ErrInvalidLength  error = errors.New("invalid length read")
)

func validateType(t byte) (packetType, error) {
	pt := packetType(t)
	switch pt {
	case
		connectcmd,
		connackcmd,
		publishcmd,
		pubackcmd,
		pubreccmd,
		pubrelcmd,
		pubcompcmd,
		subscribecmd,
		subackcmd,
		unsubscribecmd,
		unsubackcmd,
		pingreqcmd,
		pingrespcmd,
		disconnectcmd,
		authcmd:
		return pt, nil
	default:
		return 0x00, ErrInvalidCommand
	}
}

func newPacket(buff []byte) (*packet, error) {
	cmd := buff[0] & 0xf0
    flags := buff[0] & 0x0f

	pt, err := validateType(cmd)
	if err != nil {
		return nil, err
	}

	length, err := decodeVarint(buff[1:])
	if err != nil {
		return nil, err
	}

	remainingLen := evalBytes(length)
	return &packet{
		cmd:    pt,
		flags:  flags,
		buffer: bytes.NewBuffer(buff[remainingLen:length]),
		length: remainingLen,
	}, nil
}

func (pkt *packet) readByte() (byte, error) {
	return pkt.buffer.ReadByte()
}

func (pkt *packet) readVarint() (uint32, error) {
	v, err := decodeVarint(pkt.buffer.Bytes())
	if err != nil {
		return 0, err
	}

	_ = pkt.buffer.Next(evalBytes(v))
	return v, nil
}

func (pkt *packet) readString() (string, error) {
	s, err := readUTFString(pkt.buffer.Bytes())
	if err != nil {
		return "", err
	}

	_ = pkt.buffer.Next(2 + len(s))

	return s, nil
}

func (pkt *packet) readUint32() (uint32, error) {
	ui, err := readUint32(pkt.buffer.Bytes())
	if err != nil {
		return 0, err
	}

	_ = pkt.buffer.Next(4)
	return ui, nil
}

func (pkt *packet) readUint16() (uint16, error) {
	ui, err := readUint16(pkt.buffer.Bytes())
	if err != nil {
		return 0, err
	}

	_ = pkt.buffer.Next(2)
	return ui, nil
}

func (pkt *packet) readProperties(max int) ([]property, error) {
	properties := make([]property, 0, max)
	propsLen, err := pkt.readVarint()
	if err != nil {
		return nil, err
	}

	if int(propsLen) > pkt.buffer.Len() {
		return nil, ErrInvalidLength
	}

	for (pkt.buffer.Len() - int(propsLen)) > 0 {
		prop, err := readProperty(pkt)
		if err != nil {
			return nil, err
		}
		properties = append(properties, prop)
	}
	return properties, nil
}
