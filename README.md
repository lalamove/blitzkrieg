Blitskrige
==========
Blitskrige is a refactoring of [Dave Cheney](https://github.com/dave)'s work on [blast](https://github.com/dave/blast/).

Blitskrige builds solid foundation for building custom load testers with custom logic and behaviour yet with the 
statistical foundation provided in [blast](https://github.com/dave/blast/), for extensive details on the performance
of a API target. 

 * Blitskrige makes API requests at a fixed rate.
 * The number of concurrent workers is configurable.
 * The worker API allows custom protocols or logic for how a target get's tested
 * Blitskrige builds a solid foundation for custom load testers.

 ## From source
 ```
 go get -u github.com/gokit/blitskrige
 ```

 Status
 ======

 Blitskrige prints a summary every ten seconds. While Blitskrige is running, you can hit enter for an updated
 summary, or enter a number to change the sending rate. Each time you change the rate a new column
 of metrics is created. If the worker returns a field named `status` in it's response, the values
 are summarised as rows.

 Here's an example of the output:

 ```
 Metrics
 =======
 Concurrency:      1999 / 2000 workers in use

 Desired rate:     (all)        10000        1000         100
 Actual rate:      2112         5354         989          100
 Avg concurrency:  1733         1976         367          37
 Duration:         00:40        00:12        00:14        00:12

 Total
 -----
 Started:          84525        69004        14249        1272
 Finished:         82525        67004        14249        1272
 Mean:             376.0 ms     374.8 ms     379.3 ms     377.9 ms
 95th:             491.1 ms     488.1 ms     488.2 ms     489.6 ms

 200
 ---
 Count:            79208 (96%)  64320 (96%)  13663 (96%)  1225 (96%)
 Mean:             376.2 ms     381.9 ms     374.7 ms     378.1 ms
 95th:             487.6 ms     489.0 ms     487.2 ms     490.5 ms

 404
 ---
 Count:            2467 (3%)    2002 (3%)    430 (3%)     35 (3%)
 Mean:             371.4 ms     371.0 ms     377.2 ms     358.9 ms
 95th:             487.1 ms     487.1 ms     486.0 ms     480.4 ms

 500
 ---
 Count:            853 (1%)     685 (1%)     156 (1%)     12 (1%)
 Mean:             371.2 ms     370.4 ms     374.5 ms     374.3 ms
 95th:             487.6 ms     487.1 ms     488.2 ms     466.3 ms

 Current rate is 10000 requests / second. Enter a new rate or press enter to view status.

 Rate?
 ```



Workers
=======

Worker is an interface that allows Blitskrige to support custom load testing strategies. 


```go
type LalaWorker struct {}

// Send satisfies the Worker interface.
func (e *LalaWorker) Send(ctx context.Context,  lastWctx *WorkerContext) (*WorkerContext, error) {
}

// Start satisfies the Starter interface.
func (e *LalaWorker) Start(ctx context.Context) (*WorkerContext, error) {
}

// Stop satisfies the Stopper interface.
func (e *LalaWorker) Stop(ctx context.Context) error {
}
```

Examples
========
The blitskrige package may be used to start Blitskrige from code without using the command. Here's a some 
examples of usage:

```go
ctx, cancel := context.WithCancel(context.Background())
b := blitskrige.New(ctx, cancel)
defer b.Exit()
b.SetWorker(func() blitskrige.Worker {
	return &blitskrige.ExampleWorker{
		SendFunc: func(ctx context.Context, self *blitskrige.ExampleWorker, in map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"status": 200}, nil
		},
	}
})
b.Headers = []string{"header"}
b.SetData(strings.NewReader("foo\nbar"))
stats, err := b.Start(ctx)
if err != nil {
	fmt.Println(err.Error())
	return
}
fmt.Printf("Success == 2: %v\n", stats.All.Summary.Success == 2)
fmt.Printf("Fail == 0: %v", stats.All.Summary.Fail == 0)
// Output:
// Success == 2: true
// Fail == 0: true
```

```go
ctx, cancel := context.WithCancel(context.Background())
b := blitskrige.New(ctx, cancel)
defer b.Exit()
b.SetWorker(func() blitskrige.Worker {
	return &blitskrige.ExampleWorker{
		SendFunc: func(ctx context.Context, self *blitskrige.ExampleWorker, in map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"status": 200}, nil
		},
	}
})
b.Rate = 1000
wg := &sync.WaitGroup{}
wg.Add(1)
go func() {
	stats, err := b.Start(ctx)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("Success > 10: %v\n", stats.All.Summary.Success > 10)
	fmt.Printf("Fail == 0: %v", stats.All.Summary.Fail == 0)
	wg.Done()
}()
<-time.After(time.Millisecond * 100)
b.Exit()
wg.Wait()
// Output:
// Success > 10: true
// Fail == 0: true
```
 
