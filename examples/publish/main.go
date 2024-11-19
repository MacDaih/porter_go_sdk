package main

import (
	"context"
	"fmt"
	"os"
	"time"

	sdk "github.com/macdaih/porter_go_sdk"
)

func main() {

	client := sdk.NewClient(
		os.Getenv("SERVER_ADDR"),
		10,
		sdk.QoSOne,
		900,
		sdk.WithTimeout(10),
		sdk.WithID("bb476565-c9c3-4f17-bb04-686d57bf1859"),
		sdk.WithBasicCredentials("test", "test"),
		sdk.WithCallBack(
			func(_ context.Context, payload []byte) error {
				fmt.Println(string(payload))
				return nil
			},
		),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(120)*time.Second)
	defer cancel()

	msg := sdk.AppMessage{
		MessageQoS:  sdk.QoSZero,
		TopicName:   "/app_telemetry/",
		Format:      true,
		ContentType: sdk.Json,
		Payload:     []byte("{\"data\": { \"value\": 100}}"),
	}
	if err := client.Publish(ctx, msg); err != nil {
		panic(err)
	}

}
