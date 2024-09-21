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
		5,
		sdk.QoSOne,
		sdk.WithID("bb476565-c9c3-4f17-bb04-686d57bf1859"),
		sdk.WithBasicCredentials("test", "test"),
		sdk.WithCallBack(
			func(_ context.Context, payload []byte) error {
				fmt.Println(string(payload))
				return nil
			},
		),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(20)*time.Second)
	defer cancel()

	if err := client.Subscribe(ctx, []string{"outside/weather"}); err != nil {
		panic(err)
	}

}
