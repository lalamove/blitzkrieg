package blast

import (
	"context"
	"fmt"
	"github.com/francoispqt/gojay"
	"io"
	"os"
	"os/signal"
	"sync"

	"time"

	"sync/atomic"

	"github.com/leemcloughlin/gofarmhash"
	"github.com/pkg/errors"
)

// Set debug to true to print the number of active goroutines with every status.
const debug = false


var (
	// ErrWorkerNotFinished is returned when worker finished processing and response is not yet received.
	ErrWorkerNotFinished = errors.New("worker response not yet received")

	// ErrWorkerFinished is returned when worker is finished processing and response was received therefore not
	// allowing changes to be made to it's context.
	ErrWorkerFinished = errors.New("worker response already received")
)

// WorkerFunc defines a function type which generates a new Worker for running a load test.
type WorkerFunc func() Worker


// Config provides all the standard config options. Use the Initialise method to configure with a provided Config.
type Config struct {
	// WorkerFunc is responsible for generating workers for a load-test suite.
	WorkerFunc WorkerFunc

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

	// Log sets the filename of the log file to create / append to.
	Log io.Writer

	// Rate sets the initial rate in requests per second.
	// (Default: 10 requests / second).
	Rate float64

	// Workers sets the number of concurrent workers. (Default: 10 workers).
	Workers int

	// Timeout sets the deadline in the context passed to the worker. Workers must respect this the context cancellation.
	// We exit with an error if any worker is processing for timeout + 1 second. (Default: 1 second).
	Timeout time.Duration
}

// Blaster provides the back-end blast: a simple tool for API load testing and batch jobs. Use the New function to create a Blaster with default values.
type Blaster struct {
	config *Config

	softTimeout time.Duration
	hardTimeout time.Duration
	skip        map[farmhash.Uint128]struct{}

	logWriter  io.Writer
	logCloser  io.Closer
	outWriter  io.Writer
	outCloser  io.Closer
	dataCloser io.Closer

	cancel context.CancelFunc

	mainChannel            chan int
	errorChannel           chan error
	workerChannel          chan workDef
	dataFinishedChannel    chan struct{}
	workersFinishedChannel chan struct{}
	itemFinishedChannel    chan struct{}
	changeRateChannel      chan float64
	signalChannel          chan os.Signal

	mainWait   *sync.WaitGroup
	workerWait *sync.WaitGroup

	errorsIgnored uint64
	metrics       *metricsDef
	err           error
}


// New creates a new Blaster with defaults.
func New(ctx context.Context, cancel context.CancelFunc) *Blaster {
	b := &Blaster{
		cancel:                 cancel,
		mainWait:               new(sync.WaitGroup),
		workerWait:             new(sync.WaitGroup),
		skip:                   make(map[farmhash.Uint128]struct{}),
		dataFinishedChannel:    make(chan struct{}),
		workersFinishedChannel: make(chan struct{}),
		changeRateChannel:      make(chan float64, 1),
		errorChannel:           make(chan error),
		mainChannel:            make(chan int),
		workerChannel:          make(chan workDef),
		softTimeout:            time.Second,
		hardTimeout:            time.Second * 2,
	}

	// trap Ctrl+C and call cancel on the context
	b.signalChannel = make(chan os.Signal, 1)
	signal.Notify(b.signalChannel, os.Interrupt)
	go func() {
		select {
		case <-b.signalChannel:
			// notest
			b.cancel()
		case <-ctx.Done():
		}
	}()

	return b
}

// Exit cancels any goroutines that are still processing, and closes all files.
func (b *Blaster) Exit() {
	if b.logCloser != nil {
		_ = b.logCloser.Close() // ignore error
	}
	if b.outCloser != nil {
		_ = b.outCloser.Close() // ignore error
	}
	if b.dataCloser != nil {
		_ = b.dataCloser.Close() // ignore error
	}
	signal.Stop(b.signalChannel)
	b.cancel()
}

// Start starts the blast run without processing any config.
func (b *Blaster) Start(ctx context.Context, c Config) (Stats, error) {
	if err := b.initialiseConfig(c); err != nil {
		return Stats{}, err
	}
	
	b.metrics = newMetricsDef(b.config)
	err := b.start(ctx)
	return b.Stats(), err
}

// Stats returns a snapshot of the metrics (as is printed during interactive execution).
func (b *Blaster) Stats() Stats {
	return b.metrics.stats()
}

func (b *Blaster) start(ctx context.Context) error {
	b.metrics.addSegment(b.config.Rate)

	b.startTickerLoop(ctx)
	b.startMainLoop(ctx)
	b.startErrorLoop(ctx)
	b.startWorkers(ctx)

	// wait for cancel or finished
	select {
	case <-ctx.Done():
	case <-b.dataFinishedChannel:
	}

	b.println("Waiting for workers to finish...")
	b.workerWait.Wait()
	b.println("All workers finished.")

	// signal to log and error loop that it's tine to exit
	close(b.workersFinishedChannel)

	b.println("Waiting for processes to finish...")
	b.mainWait.Wait()
	b.println("All processes finished.")

	if b.err != nil {
		b.println("")
		errorsIgnored := atomic.LoadUint64(&b.errorsIgnored)
		if errorsIgnored > 0 {
			b.printf("%d errors were ignored because we were already exiting with an error.\n", errorsIgnored)
		}
		b.printf("Fatal error: %v\n", b.err)
		return b.err
	}
	b.println("")
	return nil
}

// initialiseConfig configures the Blaster with config options in a provided Config.
func (b *Blaster) initialiseConfig(c Config) error {
	if c.Rate <= 0 {
		c.Rate = 10
	}

	if c.Workers <= 0 {
		c.Workers = 10
	}

	if c.Timeout <= 0 {
		c.Timeout = time.Second * 1
	}

	b.config = &c
	return nil
}

// SetTimeout sets the timeout. See Config.Timeout for more details.
func (b *Blaster) SetTimeout(timeout time.Duration) {
	b.softTimeout = timeout
	b.hardTimeout = timeout + time.Second
}

// SetWorker sets the worker creation function. See httpworker for a simple example.
func (b *Blaster) SetWorker(wf func() Worker) {
	b.config.WorkerFunc = wf
}

// SetOutput sets the summary output writer, and allows the output to be redirected. The Command method sets this to os.Stdout for command line usage.
func (b *Blaster) SetOutput(w io.Writer) {
	if w == nil {
		b.outWriter = nil
		b.outCloser = nil
		return
	}
	b.outWriter = newThreadSafeWriter(w)
	if c, ok := w.(io.Closer); ok {
		b.outCloser = c
	} else {
		b.outCloser = nil
	}
}

// ChangeRate changes the sending rate during execution.
func (b *Blaster) ChangeRate(rate float64) {
	b.changeRateChannel <- rate
}

func (b *Blaster) startErrorLoop(ctx context.Context) {
	b.mainWait.Add(1)

	go func() {
		defer b.mainWait.Done()
		defer b.println("Exiting error loop")
		for {
			select {
			// don't react to ctx.Done() here because we may need to wait until workers have finished
			case <-b.workersFinishedChannel:
				// exit gracefully
				return
			case err := <-b.errorChannel:
				b.println("Exiting with fatal error...")
				b.err = err
				b.cancel()
				return
			}
		}
	}()
}

func (b *Blaster) error(err error) {
	select {
	case b.errorChannel <- err:
	default:
		// don't send to error channel if errorChannel isn't listening
		atomic.AddUint64(&b.errorsIgnored, 1)
	}
}

func (b *Blaster) startMainLoop(ctx context.Context) {
	b.mainWait.Add(1)

	go func() {
		defer b.mainWait.Done()
		defer b.println("Exiting main loop")
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.dataFinishedChannel:
				// If dataFinishedChannel is closed externally (e.g. in tests), we should return.
				return
			case segment := <-b.mainChannel:
				b.workerChannel <- workDef{segment: segment}
			}
		}
	}()
}


//********************************************************************
// println
//********************************************************************

func (b *Blaster) println(a ...interface{}) {
	if b.outWriter == nil {
		return
	}
	fmt.Fprintln(b.outWriter, a...)
}

func (b *Blaster) printf(format string, a ...interface{}) {
	if b.outWriter == nil {
		return
	}
	fmt.Fprintf(b.outWriter, format, a...)
}

//********************************************************************
// Worker Loop
//********************************************************************


func (b *Blaster) startWorkers(ctx context.Context) {
	for i := 0; i < b.config.Workers; i++ {

		w := b.config.WorkerFunc()

		var newWorkContext  *WorkerContext

		if s, ok := w.(Starter); ok {
			var moddedContext, err = s.Start(ctx)
			if err != nil {
				// notest
				b.error(errors.WithStack(err))
				return
			}

			if moddedContext == nil {
				b.error(errors.New("Worker.Start returned nil WorkerContext"))
			}

			newWorkContext = moddedContext
		}else{
			newWorkContext = WorkerContextWithoutPayload()
		}

		// Copy parameters...
		for key, param := range b.config.DefaultParams {
			if _, ok := newWorkContext.requestBody.Params[key]; !ok {
				newWorkContext.requestBody.Params[key] = param
			}
		}

		// Copy headers
		for key, header := range b.config.DefaultHeaders {
				newWorkContext.requestBody.Headers[key] = append(newWorkContext.requestBody.Headers[key], header...)
		}

		b.workerWait.Add(1)
		go func(index int) {
			defer b.workerWait.Done()
			defer func() {
				if s, ok := w.(Stopper); ok {
					if err := s.Stop(ctx); err != nil {
						// notest
						b.error(errors.WithStack(err))
						return
					}
				}
			}()

			for {
				select {
				case <-ctx.Done():
					return
				case <-b.dataFinishedChannel:
					// exit gracefully
					return
				case work := <-b.workerChannel:
					if err := b.send(ctx, w, newWorkContext, work); err != nil {
						// notest
						b.error(err)
						return
					}
					if b.itemFinishedChannel != nil {
						// only used in tests
						b.itemFinishedChannel <- struct{}{}
					}
				}
			}
		}(i)
	}
}

func (b *Blaster) send(ctx context.Context, w Worker, wctx *WorkerContext , work workDef) error {
	b.metrics.logStart(work.segment)
	b.metrics.logBusy(work.segment)
	b.metrics.busy.Inc(1)
	defer b.metrics.busy.Dec(1)

	// Record the start time
	start := time.Now()

	// Create a child context with the selected timeout
	child, cancel := context.WithTimeout(ctx, b.softTimeout)
	defer cancel()

	finished := make(chan *WorkerContext)

	success := true

	go func() {
		endingCtx, err := w.Send(child, wctx)
		if err != nil {
			success = false

			// if the endingCtx.workerErr is not set, set to returned error.
			if endingCtx.workerErr == nil {
				endingCtx.workerErr = err
			}
		}
		finished <- endingCtx
	}()

	var endCtx *WorkerContext
	var hardTimeoutExceeded bool
	select {
	case <-finished:
		// When Send finishes successfully, cancel the child context.
		cancel()
	case <-ctx.Done():
		// In the event of the main context being cancelled, cancel the child context and wait for
		// the sending goroutine to exit.
		cancel()
		select {
		case endCtx = <-finished: // notest
			// Only continue when finished channel is closed - e.g. sending goroutine has exited.
		case <-time.After(b.hardTimeout):
			hardTimeoutExceeded = true
			endCtx = wctx
		}
	case <-time.After(b.hardTimeout):
		hardTimeoutExceeded = true
	}

	if hardTimeoutExceeded {
		// If we get here then the worker is not respecting the context cancellation deadline, and
		// we should exit with an error. We don't simply log this as an unsuccessful request
		// because the sending goroutine is still running and would crete a memory leak.
		b.error(errors.New("a worker was still sending after timeout + 1 second. This indicates a bug in the worker code. Workers should immediately exit on receiving a signal from ctx.Done()"))
		return nil
	}

	b.metrics.logFinish(work.segment, endCtx.workerStatus, time.Since(start), success)

	if b.logWriter != nil {
		encoder := gojay.BorrowEncoder(b.config.Log)
		endCtx.MarshalJSONObject(encoder)
	}
	return nil
}

//********************************************************************
// Ticker Loop
//********************************************************************

func (b *Blaster) startTickerLoop(ctx context.Context) {

	b.mainWait.Add(1)

	var ticker *time.Ticker

	updateTicker := func() {
		if b.config.Rate == 0 {
			ticker = &time.Ticker{} // empty *time.Ticker will have nil C, so block forever.
			return
		}

		ticksPerSecond := b.config.Rate
		ticksPerMs := ticksPerSecond / 1000.0
		ticksPerUs := ticksPerMs / 1000.0
		ticksPerNs := ticksPerUs / 1000.0
		nsPerTick := 1.0 / ticksPerNs

		ticker = time.NewTicker(time.Nanosecond * time.Duration(nsPerTick))
	}

	changeRate := func(rate float64) {
		b.config.Rate = rate
		if ticker != nil {
			ticker.Stop()
		}
		b.metrics.addSegment(b.config.Rate)
		updateTicker()
	}

	updateTicker()

	go func() {
		defer b.mainWait.Done()
		defer b.println("Exiting ticker loop")
		defer func() {
			if ticker != nil {
				ticker.Stop()
			}
		}()
		for {

			// First wait for a tick... but we should also wait for an exit signal, data finished
			// signal or rate change command (we could be waiting forever on rate = 0).
			select {
			case <-ticker.C:
				// continue
			case <-ctx.Done():
				return
			case <-b.dataFinishedChannel:
				return
			case rate := <-b.changeRateChannel:
				// Restart the for loop after a rate change. If rate == 0, we may not want to send
				// any more.
				changeRate(rate)
				continue
			}

			segment := b.metrics.currentSegment()

			// Next send on the main channel. The channel won't have a listener if there is no idle
			// worker. In this case we should continue and log a miss.
			select {
			case b.mainChannel <- segment:
				// if main loop is waiting, send it a message
			case <-ctx.Done():
				// notest
				return
			case <-b.dataFinishedChannel:
				// notest
				return
			default:
				// notest
				// if main loop is busy, skip this tick
				continue
			}
		}
	}()
}


//********************************************************************
// threadSafeWriter
//********************************************************************

func newThreadSafeWriter(w io.Writer) *threadSafeWriter {
	return &threadSafeWriter{
		w: w,
	}
}

type threadSafeWriter struct {
	w io.Writer
	m sync.Mutex
}

// Write writes to the underlying writer in a thread safe manner.
func (t *threadSafeWriter) Write(p []byte) (n int, err error) {
	t.m.Lock()
	defer t.m.Unlock()
	return t.w.Write(p)
}

