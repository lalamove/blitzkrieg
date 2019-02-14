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
		PeriodicWrite: time.Second * 3,
		WorkerFunc: func() blitzkrieg.Worker {
			return &blitzkrieg.FunctionWorker{
				PrepareFunc: func(ctx context.Context) (workerContext *blitzkrieg.WorkerContext, e error) {
					return blitzkrieg.NewWorkerContext("hello-service", blitzkrieg.Payload{}, nil), nil
				},
				SendFunc: func(ctx context.Context, workerCtx *blitzkrieg.WorkerContext) error {
					time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))

					sub := workerCtx.FromContext("sub-service-call", blitzkrieg.Payload{}, nil)
					if err := callSecondService(sub); err != nil {
						return err
					}

					return callSecondService(workerCtx)
				},
			}
		},
	})

	if err != nil {
		fmt.Printf("Load testing ended with an error: %+s", err)
	}

	fmt.Printf("Final Stats:\n\n %+s\n", stats.String())
}

func callSecondService(workerCtx *blitzkrieg.WorkerContext) error {
	<-time.After(time.Millisecond * time.Duration(randy.Intn(100)))

	errorrand := randy.Float64()
	if errorrand > 0.99 {
		return workerCtx.SetResponse("200", blitzkrieg.Payload{}, nil)
	} else if errorrand > 0.96 {
		return workerCtx.SetResponse("400", blitzkrieg.Payload{}, errors.New("bad reqquest"))
	} else {
		return workerCtx.SetResponse("499", blitzkrieg.Payload{}, errors.New("wasted reqquest"))
	}
	return nil
}
