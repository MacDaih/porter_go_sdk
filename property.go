package portergosdk

import "errors"

type property struct {
	key   byte
	value interface{}
	size  int
}

type keyPair struct {
	key   string
	value string
}

const (
	MQTT_PROP_PAYLOAD_FORMAT_INDICATOR     = 1
	MQTT_PROP_MESSAGE_EXPIRY_INTERVAL      = 2
	MQTT_PROP_CONTENT_TYPE                 = 3
	MQTT_PROP_RESPONSE_TOPIC               = 8
	MQTT_PROP_CORRELATION_DATA             = 9
	MQTT_PROP_SUBSCRIPTION_ID              = 11
	MQTT_PROP_SESSION_EXPIRY_INTERVAL      = 17
	MQTT_PROP_ASSIGNED_CLIENT_IDENTIFIER   = 18
	MQTT_PROP_SERVER_KEEP_ALIVE            = 19
	MQTT_PROP_AUTHENTICATION_METHOD        = 21
	MQTT_PROP_AUTHENTICATION_DATA          = 22
	MQTT_PROP_REQUEST_PROBLEM_INFORMATION  = 23
	MQTT_PROP_WILL_DELAY_INTERVAL          = 24
	MQTT_PROP_REQUEST_RESPONSE_INFORMATION = 25
	MQTT_PROP_RESPONSE_INFORMATION         = 26
	MQTT_PROP_SERVER_REFERENCE             = 28
	MQTT_PROP_REASON_STRING                = 31
	MQTT_PROP_RECEIVE_MAXIMUM              = 33
	MQTT_PROP_TOPIC_ALIAS_MAXIMUM          = 34
	MQTT_PROP_TOPIC_ALIAS                  = 35
	MQTT_PROP_MAXIMUM_QOS                  = 36
	MQTT_PROP_RETAIN_AVAILABLE             = 37
	MQTT_PROP_USER_PROPERTY                = 38
	MQTT_PROP_MAXIMUM_PACKET_SIZE          = 39
	MQTT_PROP_WILDCARD_SUB_AVAILABLE       = 40
	MQTT_PROP_SUBSCRIPTION_ID_AVAILABLE    = 41
	MQTT_PROP_SHARED_SUB_AVAILABLE         = 42
)

func readProperty(pkt *packet) (property, error) {
	pkey, err := pkt.readByte()
	if err != nil {
		return property{}, err
	}

	switch pkey {
	case
		MQTT_PROP_SESSION_EXPIRY_INTERVAL,
		MQTT_PROP_MAXIMUM_PACKET_SIZE,
		MQTT_PROP_WILL_DELAY_INTERVAL,
		MQTT_PROP_MESSAGE_EXPIRY_INTERVAL:
		value, err := readUint32(pkt.buffer.Next(4))
		if err != nil {
			return property{}, err
		}

		return property{
			key:   pkey,
			value: value,
			size:  4,
		}, nil
	case
		MQTT_PROP_RECEIVE_MAXIMUM,
		MQTT_PROP_TOPIC_ALIAS_MAXIMUM,
		MQTT_PROP_TOPIC_ALIAS:
		value, err := readUint16(pkt.buffer.Next(2))
		if err != nil {
			return property{}, err
		}

		return property{
			key:   pkey,
			value: value,
			size:  2,
		}, nil
	case
		MQTT_PROP_REQUEST_PROBLEM_INFORMATION,
		MQTT_PROP_REQUEST_RESPONSE_INFORMATION,
		MQTT_PROP_PAYLOAD_FORMAT_INDICATOR,
		MQTT_PROP_MAXIMUM_QOS,
		MQTT_PROP_RETAIN_AVAILABLE,
		MQTT_PROP_WILDCARD_SUB_AVAILABLE,
		MQTT_PROP_SUBSCRIPTION_ID_AVAILABLE,
		MQTT_PROP_SHARED_SUB_AVAILABLE:
		value, err := pkt.readByte()
		if err != nil {
			return property{}, err
		}

		return property{
			key:   pkey,
			value: value,
			size:  1,
		}, nil
	case MQTT_PROP_SUBSCRIPTION_ID:
		value, err := decodeVarint(pkt.buffer.Bytes())
		if err != nil {
			return property{}, err
		}

		bsize := evalBytes(value)
		_ = pkt.buffer.Next(bsize)

		return property{
			key:   pkey,
			value: value,
			size:  bsize,
		}, nil
	case MQTT_PROP_USER_PROPERTY:
		key, err := readUTFString(pkt.buffer.Bytes())
		if err != nil {
			return property{}, err
		}
		_ = pkt.buffer.Next(len(key) + 2)

		value, err := readUTFString(pkt.buffer.Bytes())
		if err != nil {
			return property{}, err
		}
		_ = pkt.buffer.Next(len(value) + 2)

		return property{
			key:   pkey,
			value: keyPair{key: key, value: value},
			size:  len(key) + len(value),
		}, nil
	case
		MQTT_PROP_AUTHENTICATION_METHOD,
		MQTT_PROP_AUTHENTICATION_DATA,
		MQTT_PROP_CONTENT_TYPE,
		MQTT_PROP_RESPONSE_TOPIC,
		MQTT_PROP_CORRELATION_DATA,
		MQTT_PROP_SERVER_REFERENCE,
		MQTT_PROP_REASON_STRING:
		value, err := pkt.readString()
		if err != nil {
			return property{}, err
		}

		return property{
			key:   pkey,
			value: value,
			size:  len(value),
		}, nil
	default:
		return property{}, errors.New("unknown property")
	}
}
