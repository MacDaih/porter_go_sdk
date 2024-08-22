package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sdk "github.com/macdaih/porter_go_sdk"
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
		"192.168.1.185:8080",
		10,
		sdk.WithID("bb476565-c9c3-4f17-bb04-686d57bf1859"),
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(20)*time.Second)
	defer cancel()

	if err := client.Subscribe(ctx, []string{"outside/weather"}); err != nil {
		panic(err)
	}

}
