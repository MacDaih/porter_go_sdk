package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sdk "github.com/macdaih/porter-go-sdk"
)

type weatherData struct {
	Pressure    float64 `json:"pressure"`
	BoardTemp   float64 `json:"board_temp"`
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
	Lux         float64 `json:"lux"`
}

func main() {

	client := sdk.NewClient(
		"0.0.0.0:8080",
		15,
		sdk.WithID("test"),
		sdk.WithBasicCredentials("test", "test"),
		sdk.WithCallBack(
			func(payload []byte) error {
				var wd weatherData
				if err := json.Unmarshal(payload, &wd); err != nil {
					return err
				}
				fmt.Printf("temp = %6.2f\n", wd.Temperature)
				return nil
			},
		),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		panic(err)
	}

	if err := client.Subscribe([]string{"outside/weather"}); err != nil {
		panic(err)
	}

	client.Await(ctx)
}
