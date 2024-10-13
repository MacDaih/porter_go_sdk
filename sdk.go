package portergosdk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type QoS uint8

const (
	QoSZero = 0x00
	QoSOne  = 0x08
	QoSTwo  = 0x18
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

	qos QoS

	nextPacketID uint16

	creds *credential

	receivedMax     int
	sessionDuration time.Duration
	sessionExpiry   uint32
	messageHandler  func(context.Context, []byte) error

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

func WithTimeout(sec int) Option {
	return func(c *PorterClient) {
		c.sessionDuration = time.Duration(sec) * time.Second
	}
}

func NewClient(
	serverHost string,
	keepAlive uint16,
	qos QoS,
	sessionExpiry uint32,
	options ...Option,
) *PorterClient {

	if qos < 1 {
		sessionExpiry = 0
	}

	pc := PorterClient{
		serverHost:     serverHost,
		keepAlive:      keepAlive,
		receivedMax:    10,
		qos:            qos,
		sessionExpiry:  sessionExpiry,
		messageHandler: func(_ context.Context, _ []byte) error { return nil },
	}

	for _, fn := range options {
		fn(&pc)
	}

	return &pc
}

func (pc *PorterClient) connect(ctx context.Context, es chan endState) error {
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
		pc.sessionExpiry,
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
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for {
			if !pc.connOpen {
				es <- endState{}
				return
			}

			buff := make([]byte, 1024)
			if _, err := pc.conn.Read(buff); err != nil {
				e, ok := err.(net.Error)
				if ok && e.Timeout() {
					ping := []byte{0xC0, 0}
					if _, err := pc.conn.Write(ping); err != nil {
						es <- endState{err: err}
						return
					}

					if err := pc.conn.SetReadDeadline(
						time.Now().Add(time.Duration(pc.keepAlive) * time.Second),
					); err != nil {
						es <- endState{err: err}
						return
					}

					continue
				}

				if errors.Is(err, io.EOF) {
					es <- endState{}
					return
				}

				es <- endState{err: err}
				return
			}

			pc.readMessage(ctx, buff, es)
		}
	}()

	return nil
}

func (pc *PorterClient) Subscribe(ctx context.Context, topics []string) error {

	es := make(chan endState, 1)
	connCtx, cancel := withTimedContext(ctx, pc.sessionDuration)
	defer cancel()

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

	if err := pc.connect(connCtx, es); err != nil {
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
	case <-connCtx.Done():
		es <- endState{}
		return nil
	case end := <-es:
		if end.err != nil {
			if errors.Is(end.err, net.ErrClosed) {
				return nil
			}
		}
		return nil
	}
}

func (pc *PorterClient) readMessage(ctx context.Context, pkt []byte, es chan endState) {
	switch pkt[0] {
	case 0xe0:
		es <- endState{}
		return
	case 0x30:
		msg, err := readPublish(pkt)
		if err != nil {
			es <- endState{err: err}
			return
		}

		if err := pc.messageHandler(ctx, msg.Payload); err != nil {
			es <- endState{err: err}
			return
		}
	case 0x90:
		// TODO implement suback read
	}
}

func (pc *PorterClient) Publish(topic string, message any) error {
	// TODO implement
	return nil
}

func withTimedContext(ctx context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	if duration < 1 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, duration)
}
