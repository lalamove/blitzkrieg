package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/gokit/blitzkrieg"
)

var (
	randy = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func main() {

	blits := blitzkrieg.New()

	stats, err := blits.Start(context.Background(), blitzkrieg.Config{
		Segments: []blitzkrieg.HitSegment{
			{
				Rate:    10, // request each X second
				MaxHits: 50,
			},
			{
				Rate:    15, //  request per X second
				MaxHits: 100,
			},
			{
				Rate:    20, //  request per X second
				MaxHits: 1000,
			},
		},
		Metrics:       os.Stdout,
		PeriodicWrite: time.Millisecond * 300,
		WorkerFunc: func() blitzkrieg.Worker {
			return &blitzkrieg.FunctionWorker{
				SendFunc: func(ctx context.Context, workerCtx *blitzkrieg.WorkerContext) error {
					time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))

					sub := workerCtx.FromContext("second-endpoint", blitzkrieg.Payload{}, nil)

					errorrand := randy.Float64()
					if errorrand > 0.99 {
						sub.SetResponse("200", blitzkrieg.Payload{}, nil)
						return workerCtx.SetResponse("400", blitzkrieg.Payload{Body: []byte("bad request")}, errors.New(" not allowed"))
					} else if errorrand > 0.96 {
						sub.SetResponse("400", blitzkrieg.Payload{}, errors.New("bad reqquest"))
						return workerCtx.SetResponse("499", blitzkrieg.Payload{Body: []byte("waited too long")}, errors.New("request timed out"))
					} else {
						sub.SetResponse("499", blitzkrieg.Payload{}, errors.New("wasted reqquest"))
						return workerCtx.SetResponse("200", blitzkrieg.Payload{Body: []byte("Sweet!")}, nil)
					}
				},
			}
		},
	})

	if err != nil {
		fmt.Printf("Load testing ended with an error: %+s", err)
	}

	fmt.Printf("Final Stats:\n\n %+s\n", stats.String())
}
