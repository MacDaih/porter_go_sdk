package portergosdk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
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

	clientID string

	keepAlive uint16

	willFlag uint8

	cleanStart bool

	qos uint8

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
	options ...Option,
) *PorterClient {
	es := make(chan endState, 1)

	pc := PorterClient{
		serverHost:     serverHost,
		keepAlive:      keepAlive,
		endState:       es,
		receivedMax:    1,
		messageHandler: func(_ context.Context, _ []byte) error { return nil },
	}

	for _, fn := range options {
		fn(&pc)
	}

	return &pc
}

func (pc *PorterClient) connect(ctx context.Context, conn *net.TCPConn) error {

	msg, err := buildConnect(
		pc.clientID,
		pc.keepAlive,
		pc.creds,
	)

	if err != nil {
		return err
	}

	// no closed conn
	if _, err := conn.Write(msg); err != nil {
		return err
	}

	// read connack
	connbuff := make([]byte, 1024)
	if _, err := conn.Read(connbuff); err != nil {
		return err
	}

	if connbuff[0] != 0x20 {
		return fmt.Errorf("unexpected packet response code")
	}

	ka := time.Duration(pc.keepAlive) * time.Second

	go func() {
		tmark := time.Now().Add(ka)
		received := 0
		for {
			buff := make([]byte, 1024)
			if _, err := conn.Read(buff); err != nil {
				if errors.Is(err, io.EOF) {
					err = nil
				}
				pc.endState <- endState{err: err}
				return
			}

			if err := pc.readMessage(ctx, buff, &received); err != nil {
				pc.endState <- endState{err: err}
				return
			}

			if n := time.Now(); n.After(tmark) {
				tmark = n.Add(ka)
				ping := []byte{0xC0, 0}
				if _, err := conn.Write(ping); err != nil {
					pc.endState <- endState{err: err}
					return
				}
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

	defer conn.Close()

	if err := pc.connect(ctx, conn); err != nil {
		return err
	}

	msg, err := buildSubscribe(topics, 1)
	if err != nil {
		return err
	}

	if _, err := conn.Write(msg); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		pc.endState <- endState{}
		fmt.Println("ctx done")
		return nil
	case es := <-pc.endState:
		return es.err
	}
}

func (pc *PorterClient) readMessage(ctx context.Context, pkt []byte, received *int) error {
	switch pkt[0] {
	case 0xe0:
		pc.endState <- endState{err: errors.New("client disconnected")}
		return nil
	case 0x30:
		msg, err := readPublish(pkt)
		if err != nil {
			fmt.Println(err)
			return err
		}

		if err := pc.messageHandler(ctx, msg.Payload); err != nil {
			return err
		}

		*received++
		if *received >= pc.receivedMax {
			pc.endState <- endState{}
			return nil
		}
	default:
		fmt.Printf("received code %x\n", pkt[0])
	}
	return nil
}

func (pc *PorterClient) Publish(topic string, message any) error {
	// TODO implement
	return nil
}
