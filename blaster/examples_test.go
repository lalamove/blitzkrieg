package blaster_test

import (
	"context"

	"fmt"

	"strings"

	"time"

	"sync"

	"github.com/dave/blast/blaster"
)

func ExampleBlaster_Start_batchJob() {
	ctx, cancel := context.WithCancel(context.Background())
	b := blaster.New(ctx, cancel)
	defer b.Exit()
	b.SetWorker(func() blaster.Worker {
		return &blaster.ExampleWorker{
			SendFunc: func(ctx context.Context, in map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{"status": 200}, nil
			},
		}
	})
	b.Headers = []string{"header"}
	b.SetData(strings.NewReader("foo\nbar"))
	summary, err := b.Start(ctx)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("%#v", summary)
	// Output:
	// blaster.Summary{Success:2, Fail:0}
}

func ExampleBlaster_Start_loadTest() {
	ctx, cancel := context.WithCancel(context.Background())
	b := blaster.New(ctx, cancel)
	defer b.Exit()
	b.SetWorker(func() blaster.Worker {
		return &blaster.ExampleWorker{
			SendFunc: func(ctx context.Context, in map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{"status": 200}, nil
			},
		}
	})
	b.Rate = 100
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		summary, err := b.Start(ctx)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Printf("Fail: %d", summary.Fail)
		wg.Done()
	}()
	<-time.After(time.Millisecond * 100)
	b.Exit()
	wg.Wait()
	// Output:
	// Fail: 0
}