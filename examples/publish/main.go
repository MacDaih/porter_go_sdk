package main

import (
	"context"
	"os"

	sdk "github.com/macdaih/porter_go_sdk"
)

func main() {

	client := sdk.NewClient(
		os.Getenv("SERVER_ADDR"),
		1,
		sdk.QoSOne,
		1,
		sdk.WithID("bb476565-c9c3-4f17-bb04-686d57bf1859"),
		sdk.WithBasicCredentials("test", "test"),
	)

	msg := sdk.AppMessage{
		MessageQoS: sdk.QoSZero,
		TopicName:  "app_telemetry",
		Format:     true,
		Content:    sdk.Json,
		Payload:    []byte(`{"data": {"value": 100}, "text_field": "lorem ipsum blablaba", "amount":100.00}`),
	}
	if err := client.Publish(context.Background(), msg); err != nil {
		panic(err)
	}

}
