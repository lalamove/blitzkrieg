[![Build Status](https://travis-ci.org/gokit/blitzkrieg.svg?branch=master)](https://travis-ci.org/gokit/blitzkrieg) [![Go Report Card](https://goreportcard.com/badge/github.com/lalamove/blitzkrieg)](https://goreportcard.com/report/github.com/lalamove/blitzkrieg) [![codecov](https://codecov.io/gh/gokit/blitzkrieg/branch/master/graph/badge.svg)](https://codecov.io/gh/gokit/blitzkrieg)

Blitzkrieg
==========
Blitzkrieg is a refactoring of [Dave Cheney](https://github.com/dave)'s work on [blast](https://github.com/dave/blast/).

Blitzkrieg builds solid foundation for building custom load testers with custom logic and behaviour yet with the 
statistical foundation provided in [blast](https://github.com/dave/blast/), for extensive details on the performance
of a API target. 

 * Blitzkrieg makes API requests at a fixed rate.
 * The number of concurrent workers is configurable.
 * The worker API allows custom protocols or logic for how a target get's tested
 * Blitzkrieg builds a solid foundation for custom load testers.

 ## From source
 
 ```
 go get -u github.com/lalamove/blitzkrieg
 ```

 Status
 ======

 Blitzkrieg prints a summary every ten seconds. While Blitzkrieg is running, you can hit enter for an updated
 summary, or enter a number to change the sending rate. Each time you change the rate a new column
 of metrics is created. If the worker returns a field named `status` in it's response, the values
 are summarised as rows.

 Here's an example of the output: (See [Hello](./examples/hello/main.go) for code)

```
 
====================================================
Metrics
====================================================
Concurrency:                          0 / 10 workers in use

Desired rate:                         (all)       20         15        10
Actual rate:                          19          20         15        10
Avg concurrency:                      4           5          3         2
Duration:                             01:02       00:50      00:06     00:05

Total
-----
Started:                              1149        999        100       50
Finished:                             1149        999        100       50
Success:                              9           6          2         1
Fail:                                 1140        993        98        49
Mean:                                 252.0 ms    250.0 ms   260.0 ms  277.0 ms
95th:                                 404.0 ms    404.0 ms   394.0 ms  425.0 ms

hello-service (200)
-------------------
Count:                                9 (1%)      6 (1%)     2 (2%)    1 (2%)
Mean:                                 309.0 ms    289.0 ms   349.0 ms  352.0 ms
95th:                                 391.0 ms    357.0 ms   391.0 ms  352.0 ms

hello-service (400)
-------------------
Count:                                43 (4%)     31 (3%)    8 (8%)    4 (8%)
Mean:                                 260.0 ms    259.0 ms   241.0 ms  310.0 ms
95th:                                 408.0 ms    415.0 ms   375.0 ms  380.0 ms

hello-service (499)
-------------------
Count:                                1097 (95%)  962 (96%)  90 (90%)  45 (90%)
Mean:                                 252.0 ms    250.0 ms   259.0 ms  272.0 ms
95th:                                 404.0 ms    404.0 ms   395.0 ms  428.0 ms

hello-service/sub-service-call (200)
------------------------------------
Count:                                16 (1%)     14 (1%)    1 (1%)    1 (2%)
Mean:                                 102.0 ms    102.0 ms   37.0 ms   166.0 ms
95th:                                 166.0 ms    140.0 ms   37.0 ms   166.0 ms

hello-service/sub-service-call (400)
------------------------------------
Count:                                40 (3%)     33 (3%)    5 (5%)    2 (4%)
Mean:                                 103.0 ms    98.0 ms    126.0 ms  135.0 ms
95th:                                 160.0 ms    153.0 ms   160.0 ms  176.0 ms

hello-service/sub-service-call (499)
------------------------------------
Count:                                1093 (95%)  952 (95%)  94 (94%)  47 (94%)
Mean:                                 100.0 ms    100.0 ms   104.0 ms  105.0 ms
95th:                                 166.0 ms    166.0 ms   170.0 ms  182.0 ms

====================================================


```


## Configuration

Blitzkrieg expects the following configuration values which do have set defaults during
it's use, see below:

```go
type Config struct {
	// WorkerFunc is responsible for generating workers for a load-test suite.
	WorkerFunc WorkerFunc

	// OnNextSegment sets a function to be executed once a new segment has begun.
	OnNextSegment func(HitSegment)

	// SegmentedEnded sets a function to be executed once a segment has finished.
	OnSegmentEnd func(HitSegment)

	// OnEachRun sets a function to be called on every finished execution of a giving
	// worker's request work. This way you get access to the current stat, worker id
	// and worker context used within a single instance run of a segment run.
	//
	// Note this is called for every completion of an individual request, so if you
	// set a HitSegment.MaxHits of 1000, then this would be called 1000 times.
	OnEachRun func(workerId int, workerContext *WorkerContext, stat Stats)

	// DefaultHeaders contains default headers that all workers must include
	// in their requests.
	//
	// All header values are copied/appended into the initial content of a worker start
	// WorkerContext, but it will append all header values into existing key
	// found in the WorkerContext returned by the Worker.Start method call.
	DefaultHeaders map[string][]string

	// DefaultParams contains default params that all workers must include
	// in their requests.
	//
	// All parameters are copied into the initial content of a worker start
	// WorkerContext, but it will not replace any key already provided for
	// if found in the WorkerContext returned by the Worker.Start method call.
	DefaultParams map[string]string

	// Log sets the io.Writer to write internal blaster logs into.
	Log io.Writer

	// Metrics sets the io.Writer to write period stats of blaster into.
	Metrics io.Writer

	// PeriodicWrite sets the intervals at which the current stats of the
	// blaster is written into the Config.Metrics writer.
	// ( Defaults: 5 second ).
	PeriodicWrite time.Duration

	// Segments sets the sampling size and total different blast segments
	// rates and max hits per segment. This allows us to provide a slice of
	// different hit rates to test targets with.
	Segments []HitSegment

	// Workers sets the number of concurrent workers. (Default: 10 workers).
	Workers int

	// Timeout sets the deadline in the context passed to the worker. Workers must respect this the context cancellation.
	// We exit with an error if any worker is processing for timeout + 1 second.
	// (Default: 1 second).
	Timeout time.Duration

	// Endless sets whether the blaster should still continue working, if it has exhausted
	// it's segments list. This allows us dynamically deliver new segments to be delivered
	// into a current running blaster.
	Endless bool
}
```

## HitSegments

Blitzkrieg provides [HitSegments](https://github.com/lalamove/blitzkrieg/blob/master/blaster.go#L39-L48), which are you means 
of configuring how your load test workers are used. Each HitSegment is basically a giving rate of maximum 
requests we wish to hit against our target, where Blitzkrieg will collect the response, and behaviour statistics
of our target during such a segment. This allows us create a list of desirable and non-desirable hit segments which 
can be used to test the behaviour of our target during what we consider a healthy rate or an unhealthy rate.

```go
type HitSegment struct {
	// Rate sets the initial rate in requests per second.
	// (Default: 10 requests / second).
	Rate float64

	// MaxHits sets the maximum amount of hits per the rate value
	// which this segment will run.
	// (Defaults: 1000).
	MaxHits int
}
```

It provides 3 configuration parameters:

- `Rate` sets the maximum number of request is to be made per second

- `MaxHits` sets the maximum number of hits we wish to make against a target.

Where once the count of `MaxHits` as being made, then a `HitSegment` is considered done.



## Worker API

Blitzkrieg Worker interface allows Blitzkrieg to support your custom load testing strategies. 

The workers used by blitskrieg for your load test is generated by you when you provide 
the following function, which returns new instances for use in concurrently load testing 
their internal target. 

*In blitzkrieg, you handle the target you wish to target in your worker and the data they 
will be using for their tests. Remember blitzkrieg is a foundation, it collects the stats
for you, so you just extend it for custom load testing setup.*

```go
func sampleWorker() blitzkriege.Worker {
	return &LalaWorker{}
}
```


### Worker Examples

See Worker interface implementation examples below:

#### Single Request Worker  

This is where a single request is to be made to hit at our target as defined by us. This worker
will be called to repeatedly prepare it's payload and then we measure how long it takes for it 
to make it's request and get it's response.

```go
type LalaServiceConfig struct{
	MainServiceAddr string
	MainServicePort int
	TlsCert *tls.Cert
}

type LalaWorker struct {}

// Send will contain all necessary call or calls require for your load testing
// You can use the WorkerContext.FromContext method to create a child context to 
// detail the response, status and error that occurs from that sub-request, this then
// allows us follow that tree to create comprehensive statistics for your load test.
func (e *LalaWorker) Send(ctx context.Context,  workerContext *WorkerContext) error {
	
	// Call target service and record response, err and status.
	resStatus, response, err := callMainService(ctx)
	
	workerContext.SetResponse(resStatus, Payload{ Body: response }, err)
	return err
}

// Prepare Prepare should be where you load the sample data you wish to test your worker with.
// You might need to implement some means of selectively or randomly loading different
// data for your load tests here, as Blitzkrieg won't handle that for you.
//
// This is called on every time a worker is to be executed, so you get the chance
// to prepare the payload data, headers and parameters you want in a request.
func (e *LalaWorker) Prepare(ctx context.Context) (*WorkerContext, error) {
	// Load some data from disk or some remote service
	var customBody, err = LoadData()
	if err != nil {
		return nil, err
	}
	
	// You can add custom parameters and headers or load this all from 
	// some custom encoded file (e.g in JSON).
	var customParameters = map[string]string{"user_id": "some_id"}
	var customHeader = map[string][]string{
		"X-Record-Meta": []string{"raf-4334", "xaf-rt"},
	}
	
	
	// create custom request payload.
	var payload = blitzkrieg.Payload{
		Body: customBody, 
		Params: customParameters, 
		Headers: customHeader,
	}
	
	// create or load some custom meta data or config for worker.
	var serviceMeta = LalaServiceConfig{}
	
	return blitskrieg.NewWorkerContext("raf-api-test", payload, serviceMeta)
}

// Start handle some base initialization logic you wish to be done before worker use.
//
// Remember Blitskrieg will create multiple versions of this worker with the 
// register WorkerFunc, so don't cause race conditions.
func (e *LalaWorker) Start(ctx context.Context) error {
	
	return nil
}

// You handle whatever cleanup you wish to be done for this worker.
//
// Remember Blitskrieg will create multiple versions of this worker with the 
// register WorkerFunc, so don't cause race conditions.
func (e *LalaWorker) Stop(ctx context.Context) error {
	// do something....
}
```

#### Group/Sequence Request Worker

This is where your load test spans multiple requests, where each makes up the single operation 
you wish to validate it's behaviour, using the `WorkerContext.FromContext` which branches of 
child WorkerContext, we can ensure the worker and this sub request data are measured and 
and aggregated.

```go
type LalaServiceConfig struct{
	MainServiceAddr string
	MainServicePort int
	TlsCert *tls.Cert
}

type LalaWorker struct {}

// Send will contain all necessary call or calls require for your load testing
// You can use the WorkerContext.FromContext method to create a child context to 
// detail the response, status and error that occurs from that sub-request, this then
// allows us follow that tree to create comprehensive statistics for your load test.
func (e *LalaWorker) Send(ctx context.Context,  workerContext *WorkerContext) error {
	
	// Make request 1 to first API endpoint using child context.
	firstWorkerContext  := workerContext.FromContext("request-1", Payload{})
	if err := callFirstService(ctx, secondWorkerContext); err != nil {
		return err
	}
	
	// Make request 2 to next API endpoint in series
	secondWorkerContext  := workerContext.FromContext("request-2", Payload{})
	if err := callSecondService(ctx, secondWorkerContext); err != nil {
		return err
	}
	
	// Make main request with worker context
	resStatus, response, err := callMainService(ctx)
	 
	workerContext.SetResponse(resStatus, Payload{ Body: response }, err)
	return err
}

// Prepare Prepare should be where you load the sample data you wish to test your worker with.
// You might need to implement some means of selectively or randomly loading different
// data for your load tests here, as Blitzkrieg won't handle that for you.
//
// This is called on every time a worker is to be executed, so you get the chance
// to prepare the payload data, headers and parameters you want in a request.
func (e *LalaWorker) Prepare(ctx context.Context) (*WorkerContext, error) {
	// Load some data from disk or some remote service
	var customBody, err = LoadData()
	if err != nil {
		return nil, err
	}
	
	// You can add custom parameters and headers or load this all from 
	// some custom encoded file (e.g in JSON).
	var customParameters = map[string]string{"user_id": "some_id"}
	var customHeader = map[string][]string{
		"X-Record-Meta": []string{"raf-4334", "xaf-rt"},
	}
	
	
	// create custom request payload.
	var payload = blitzkrieg.Payload{
		Body: customBody, 
		Params: customParameters, 
		Headers: customHeader,
	}
	
	// create or load some custom meta data or config for worker.
	var serviceMeta = LalaServiceConfig{}
	
	return blitskrieg.NewWorkerContext("raf-api-test", payload, serviceMeta)
}

// Start handle some base initialization logic you wish to be done before worker use.
//
// Remember Blitskrieg will create multiple versions of this worker with the 
// register WorkerFunc, so don't cause race conditions.
func (e *LalaWorker) Start(ctx context.Context) error {
	
	return nil
}

// You handle whatever cleanup you wish to be done for this worker.
//
// Remember Blitskrieg will create multiple versions of this worker with the 
// register WorkerFunc, so don't cause race conditions.
func (e *LalaWorker) Stop(ctx context.Context) error {
	// do something....
}
```

## Example


```go
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
```