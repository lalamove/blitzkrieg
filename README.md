[![Build Status](https://travis-ci.org/lalamove/blitzkrieg.svg?branch=master)](https://travis-ci.org/lalamove/blitzkrieg) 
[![Go Report Card](https://goreportcard.com/badge/github.com/lalamove/blitzkrieg)](https://goreportcard.com/report/github.com/lalamove/blitzkrieg) 
[![codecov](https://codecov.io/gh/lalamove/blitzkrieg/branch/master/graph/badge.svg)](https://codecov.io/gh/lalamove/blitzkrieg)
[![Go doc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/lalamove/blitzkrieg)

Blitzkrieg
==========
Blitzkrieg is a refactoring of [Dave](https://github.com/dave)'s work on [blast](https://github.com/dave/blast/).

Blitzkrieg builds solid foundation for building custom load testers with custom logic and behaviour yet with the 
statistical foundation provided in [blast](https://github.com/dave/blast/), for extensive details on the performance
of a API target. 

 * Blitzkrieg makes API requests at a fixed rate.
 * The number of concurrent workers is configurable.
 * The worker API allows custom protocols or logic for how a target get's tested

## From source

```
go get -u github.com/lalamove/blitzkrieg
```

## Example


```go
blits := blitzkrieg.New()

stats, err := blits.Start(context.Background(), blitzkrieg.Config{
	Segments: []blitzkrieg.HitSegment{
		{
			Rate:    1000, // request X per second
			MaxHits: 50,
		},
		{
			Rate:    1500, //  request X per second
			MaxHits: 100,
		},
		{
			Rate:    2000, //  request X per second
			MaxHits: 1000,
		},
	},
	Workers: 200,
	Metrics:       os.Stdout,
	PeriodicWrite: time.Second * 3,
	WorkerFunc: func() blitzkrieg.Worker {
		return &blitzkrieg.FunctionWorker{
			PrepareFunc: func(ctx context.Context) (workerContext *blitzkrieg.WorkerContext, e error) {
				return blitzkrieg.NewWorkerContext("hello-service", blitzkrieg.Payload{}, nil), nil
			},
			SendFunc: func(ctx context.Context, workerCtx *blitzkrieg.WorkerContext)  {
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))

				sub := workerCtx.FromContext("sub-service-call", blitzkrieg.Payload{}, nil)
				if err := callSecondService(sub); err != nil {
					return 
				}

				callSecondService(workerCtx)
			},
		}
	},
})

if err != nil {
	fmt.Printf("Load testing ended with an error: %+s", err)
}

fmt.Printf("Final Stats:\n\n %+s\n", stats.String())
```

### Status

Blitzkrieg prints a summary through it's `Stats` object, which is accessible 
during each worker's call or at the end of all hit segments. 

Here's an example of the output: (See [Hello](./examples/hello/main.go) for code)

Because Blitzkrieg uses the `status` text provided as a collation point, you will see
the aggregated facts under each title of service with it's status. 

```

====================================================
Metrics
====================================================
Concurrency:                          0 / 200 workers in use

Desired rate (Per Second):            (all)       2000        1500       1000     ( Set Rate for each HitSegment )
Actual rate  (Per Second):            335         295         368        437      ( Actual Observed rate of attack )
Avg concurrency (Active):             194         199         199        159      ( Average active workers in each segment )
Duration:                             00:08       00:05       00:02      00:01    ( Duration in seconds taking to complete segment )

Total
-----
Started:                              2999        1499        1000       500
Finished:                             2999        1499        1000       500
Success:                              29          17          8          4
Fail:                                 2970        1482        992        496
Mean:                                 545.0 ms    553.0 ms    540.0 ms   553.0 ms
95th:                                 952.0 ms    952.0 ms    955.0 ms   958.0 ms

hello-service (200)
-------------------
Count:                                29 (1%)     17 (1%)     8 (1%)     4 (1%)
Mean:                                 497.0 ms    420.0 ms    643.0 ms   534.0 ms
95th:                                 973.0 ms    990.0 ms    919.0 ms   956.0 ms

hello-service (400)
-------------------
Count:                                85 (3%)     45 (3%)     22 (2%)    18 (4%)
Mean:                                 590.0 ms    597.0 ms    577.0 ms   588.0 ms
95th:                                 1004.0 ms   1004.0 ms   1013.0 ms  987.0 ms

hello-service (499)
-------------------
Count:                                2885 (96%)  1437 (96%)  970 (97%)  478 (96%)
Mean:                                 552.0 ms    553.0 ms    538.0 ms   552.0 ms
95th:                                 961.0 ms    958.0 ms    955.0 ms   950.0 ms

hello-service/sub-service-call (200)
------------------------------------
Count:                                17 (1%)     3 (0%)      11 (1%)    3 (1%)
Mean:                                 107.0 ms    87.0 ms     108.0 ms   127.0 ms
95th:                                 178.0 ms    112.0 ms    178.0 ms   167.0 ms

hello-service/sub-service-call (400)
------------------------------------
Count:                                98 (3%)     52 (3%)     32 (3%)    14 (3%)
Mean:                                 96.0 ms     96.0 ms     96.0 ms    99.0 ms
95th:                                 167.0 ms    169.0 ms    172.0 ms   166.0 ms

hello-service/sub-service-call (499)
------------------------------------
Count:                                2884 (96%)  1444 (96%)  957 (96%)  483 (97%)
Mean:                                 97.0 ms     101.0 ms    98.0 ms    99.0 ms
95th:                                 163.0 ms    169.0 ms    162.0 ms   171.0 ms

====================================================


```


## Configuration

Blitzkrieg provides the `Config` type with it's field which may have defaults for it's operations.

```go
type Config struct {
	// WorkerFunc is responsible for generating workers for load-testing.
	WorkerFunc WorkerFunc

	// OnNextSegment sets a function to be executed once a new hit segment has begun.
	OnNextSegment func(HitSegment)

	// SegmentedEnded sets a function to be executed once a hit segment has finished.
	OnSegmentEnd func(HitSegment)

	// OnEachRun sets a function to be called on every finished execution of a giving
	// worker's request. This way you get access to the current Stats, worker id
	// and worker context used within a single run of a hit segment.
	//
	// Note this is called for every completion of an individual request, so if you
	// set a HitSegment.MaxHits of 1000, then this would be called 1000 times.
	OnEachRun func(workerId int, workerContext *WorkerContext, stat Stats)

	// DefaultHeaders contains default headers that all workers must include
	// in their requests.
	//
	// All header values are copied/appended into the initial content of a worker 
	// WorkerContext, if an existing header is found then it will append default 
	// header values into key header list.
	//
	// The header is also returned by the call to Worker.Prepare.
	DefaultHeaders map[string][]string

	// DefaultParams contains default params that all workers must include
	// in their requests.
	//
	// All parameters are copied into the initial content of a worker start
	// WorkerContext, but it will not replace any key already provided for,
	// if found in the WorkerContext returned by the Worker.Prepare method call.
	DefaultParams map[string]string

	// Log sets the Writer to write internal blaster logs into.
	Log io.Writer

	// Metrics sets the Writer to write periodic stats of blaster into.
	Metrics io.Writer

	// PeriodicWrite sets the intervals at which the current stats of the
	// blaster is written into the Config.Metrics writer.
	// ( Defaults: 5 second ).
	PeriodicWrite time.Duration

	// Segments sets the sampling size and total different blast segments
	// rates and max hits which will be used for each worker.
	Segments []HitSegment

	// Workers sets the number of concurrent workers to be used. 
	// (Default: 10 workers).
	Workers int

	// Timeout sets the deadline in the context passed to the worker. Workers must 
	// respect the context cancellation.
	// We exit with an error if any worker is processing for timeout + 1 second.
	// (Default: 1 second).
	Timeout time.Duration

	// Endless sets whether the blaster should still continue working, if it has exhausted
	// it's segments list. This allows us dynamically deliver new segments to be delivered
	// into a current running blaster.
	Endless bool
}
```

Note: The amount of `Workers` sets in `Config.Workers` has a direct effect on the rates of request
per second able to be made, so ensure the worker count is more than the maximum rate per second you have
in all your hit segments.

## HitSegments

[HitSegments](https://github.com/lalamove/blitzkrieg/blob/master/blaster.go#L39-L48) are the means by which 
you designate how you wish to attack your target. 

It defines the number of request per second rate and the maximum request to be sent to your target through the use of 
your worker. Blitzkrieg does not provide a means of delivery data to each worker, but rather let's you decide how that 
will be performed through the worker's `Prepare` method, more so, your worker also decides how it will perform it's attack
through it's `Send` method as well.

Use `HitSegments` to design your attacking threshold for what you would consider a healthy or non-healthy
rate for your service to see how it behaves as such points of load.

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

It provides 2 configuration parameters:

- `Rate` sets the maximum number of request is to be made per second

- `MaxHits` sets the maximum number of hits we wish to make against a target at the allowed rate.

Where once the count of `MaxHits` as being made, then a `HitSegment` is considered done.


Blitzkrieg will cycle through all available `HitSegments` one by one till there is no more available.


## Worker API

To support custom data loading and protocol attacks, Blitzkrieg exposes the Worker interface:  

```go
type Worker interface {
	// Prepare handles the loading of the initial data a worker should begin with, it is
	// responsible for creating what data a request should be or contain.
	Prepare(ctx context.Context) (*WorkerContext, error)

	// Send handles the internal internal logic necessary for attacking the target end 
	Send(ctx context.Context, workerCtx *WorkerContext) 
}

```

Your Worker is expected to implement the `Prepare` method which will be called every time a hit is to 
be made to prepare the data it will use within the returned `WorkerContext`. The `WorkerContext` provides 
the means by which the request payload is delivered to the `Worker.Send` method and also by which we
get the `response`, `error` and `status` back through the `WorkerContext.SetResponse` from giving worker attack run.

The `Worker` implementation can also implement the `Starter` and `Stopper` interfaces to provide initialization and 
tear down operations to be called when such a worker is being closed down.

```go
// Starter is an interface a worker can optionally satisfy to provide initialization logic.
type Starter interface {
	Start(ctx context.Context) error
}

// Stopper is an interface a worker can optionally satisfy to provide finalization logic.
type Stopper interface {
	Stop(ctx context.Context) error
}
```

Blitzkrieg expects you to provide a `Worker` generating function to it's `Config.Worker` field for use in 
creating workers during it's run. 


```go
func SampleWorkerGenerator() blitzkriege.Worker {
	return &LalaWorker{}
}
```


### Worker Examples

Because of the flexibility of the `Worker` interface, you have the freedom to decide what type of operation or
set of operations are considered to be a single target attack. This let's you create load testing workloads that 
span multiple or single calls easily.

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
func (e *LalaWorker) Send(ctx context.Context,  workerContext *WorkerContext)  {
	
	// Call target service and record response, err and status.
	resStatus, response, err := callMainService(ctx)
	
	workerContext.SetResponse(resStatus, Payload{ Body: response }, err)
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
	
	return blitskrieg.NewWorkerContext("raf-api-test", payload, serviceMeta), nil
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
func (e *LalaWorker) Send(ctx context.Context,  workerContext *WorkerContext)  {
	
	// Make request 1 to first API endpoint using child context.
	firstWorkerContext  := workerContext.FromContext("request-1", Payload{})
	if err := callFirstService(ctx, secondWorkerContext); err != nil {
		workerContext.SetResponse(resStatus, Payload{ }, err)
		return 
	}
	
	// Make request 2 to next API endpoint in series
	secondWorkerContext  := workerContext.FromContext("request-2", Payload{})
	if err := callSecondService(ctx, secondWorkerContext); err != nil {
		workerContext.SetResponse(resStatus, Payload{ }, err)
		return 
	}
	
	// Make main request with worker context
	resStatus, response, err := callMainService(ctx)
	 
	workerContext.SetResponse(resStatus, Payload{ Body: response }, err)
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
	
	return blitskrieg.NewWorkerContext("raf-api-test", payload, serviceMeta), nil
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

