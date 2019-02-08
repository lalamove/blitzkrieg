Blitskrige
==========
Blitskrige is a refactoring of [Dave Cheney](https://github.com/dave)'s work on [Blast](https://github.com/dave/blast/).
It exists to provide a solid foundation for building load testers with custom logic and processing yet with the 
statistical foundation able to provide extensive details on the performance of a giving target.

 * Blast makes API requests at a fixed rate.
 * The number of concurrent workers is configurable.
 * Blast is protocol agnostic, and adding a new worker type is trivial.
 * For load testing: random data can be added to API requests.


 ## From source
 ```
 go get -u github.com/dave/blast
 ```

 Status
 ======

 Blast prints a summary every ten seconds. While blast is running, you can hit enter for an updated
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

 Config
 ======
 Blast is configured by config file, command line flags or environment variables. The `--config` flag specifies the config file to load, and can be `json`, `yaml`, `toml` or anything else that [viper](https://github.com/spf13/viper) can read. If the config flag is omitted, blast searches for `blast-config.xxx` in the current directory, `$HOME/.config/blast/` and `/etc/blast/`.

 Environment variables and command line flags override config file options. Environment variables are upper case and prefixed with "BLAST" e.g. `BLAST_PAYLOAD_TEMPLATE`.


Workers
=======

Worker is an interface that allows blast to easily be extended to support any protocol. See `main.go` for an example of how to build a command with your custom worker type.

Starter and Stopper are interfaces a worker can optionally satisfy to provide initialization or finalization logic. See `httpworker` and `dummyworker` for simple examples. 

Examples
========
The blitskrige package may be used to start blast from code without using the command. Here's a some 
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
 
