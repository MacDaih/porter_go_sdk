package portergosdk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	QoSZero = iota
	QoSOne
	QoSTwo
)

type credential struct {
	authMethod string
	usr        *string
	pwd        *string
}

type SubscribeCallback func() error

type endState struct {
	err error
}

type PorterClient struct {
	serverHost string
	conn       *net.TCPConn
	connOpen   bool

	clientID string

	keepAlive uint16

	willFlag uint8

	cleanStart bool

	qos int

	nextPacketID uint16

	creds *credential

	receivedMax    int
	messageHandler func(context.Context, []byte) error

	endState chan endState
}

type Option func(c *PorterClient)

func WithID(id string) Option {
	return func(c *PorterClient) {
		c.clientID = id
	}
}

func WithBasicCredentials(user string, pwd string) Option {
	return func(c *PorterClient) {
		c.creds = &credential{
			authMethod: PasswordMethod,
			usr:        &user,
			pwd:        &pwd,
		}
	}
}

func WithMaxMessage(max int) Option {
	return func(c *PorterClient) {
		c.receivedMax = max
	}
}

func WithCallBack(fn func(ctx context.Context, b []byte) error) Option {
	return func(c *PorterClient) {
		c.messageHandler = fn
	}
}

func NewClient(
	serverHost string,
	keepAlive uint16,
	qos int,
	options ...Option,
) *PorterClient {
	es := make(chan endState, 1)

	pc := PorterClient{
		serverHost:     serverHost,
		keepAlive:      keepAlive,
		endState:       es,
		receivedMax:    10,
		qos:            qos,
		messageHandler: func(_ context.Context, _ []byte) error { return nil },
	}

	for _, fn := range options {
		fn(&pc)
	}

	return &pc
}

func (pc *PorterClient) connect(ctx context.Context) error {
	if err := pc.conn.SetReadDeadline(
		time.Now().Add(time.Duration(pc.keepAlive) * time.Second),
	); err != nil {
		return err
	}

	msg, err := buildConnect(
		pc.clientID,
		pc.keepAlive,
		pc.creds,
		pc.qos,
	)

	if err != nil {
		return err
	}

	// no closed conn
	if _, err := pc.conn.Write(msg); err != nil {
		return err
	}

	// read connack
	connbuff := make([]byte, 1024)
	if _, err := pc.conn.Read(connbuff); err != nil {
		return err
	}

	if connbuff[0] != 0x20 {
		return fmt.Errorf("unexpected packet response code")
	}

	go func() {
		received := 0
		for {
			if !pc.connOpen {
				pc.endState <- endState{}
				return
			}

			buff := make([]byte, 1024)
			if _, err := pc.conn.Read(buff); err != nil {
				e := err.(net.Error)
				if e.Timeout() {
					ping := []byte{0xC0, 0}
					if _, err := pc.conn.Write(ping); err != nil {
						pc.endState <- endState{err: err}
						return
					}

					if err := pc.conn.SetReadDeadline(
						time.Now().Add(time.Duration(pc.keepAlive) * time.Second),
					); err != nil {
						pc.endState <- endState{err: err}
						return
					}

					continue
				}

				if errors.Is(err, io.EOF) {
					pc.endState <- endState{}
					return
				}

				pc.endState <- endState{err: err}
				return
			}

			if err := pc.readMessage(ctx, buff, &received); err != nil {
				pc.endState <- endState{err: err}
				return
			}
		}
	}()

	return nil
}

func (pc *PorterClient) Subscribe(ctx context.Context, topics []string) error {
	addr, err := net.ResolveTCPAddr("tcp4", pc.serverHost)
	if err != nil {
		return err
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}

	pc.conn = conn
	pc.connOpen = true

	defer func() {
		pc.conn.Close()
		pc.connOpen = false
	}()

	if err := pc.connect(ctx); err != nil {
		return err
	}

	msg, err := buildSubscribe(topics, 1)
	if err != nil {
		return err
	}

	if _, err := pc.conn.Write(msg); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		pc.endState <- endState{}
		return nil
	case es := <-pc.endState:
		if errors.Is(es.err, net.ErrClosed) {
			return nil
		}
		return es.err
	}
}

func (pc *PorterClient) readMessage(ctx context.Context, pkt []byte, received *int) error {
	switch pkt[0] {
	case 0xe0:
		pc.endState <- endState{}
		return nil
	case 0x30:
		msg, err := readPublish(pkt)
		if err != nil {
			return err
		}

		if err := pc.messageHandler(ctx, msg.Payload); err != nil {
			return err
		}

		*received++
		if *received >= pc.receivedMax {
			*received = 0
			pc.endState <- endState{}
			return nil
		}
	default:
	}
	return nil
}

func (pc *PorterClient) Publish(topic string, message any) error {
	// TODO implement
	return nil
}
