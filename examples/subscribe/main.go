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
		sdk.WithID("d14ce97e-f289-4439-b2e0-153b07784749"),
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

	if err := client.Subscribe(ctx, []string{"app_telemetry"}); err != nil {
		panic(err)
	}

}
