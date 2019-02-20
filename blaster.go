package blitzkrieg

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/francoispqt/gojay"

	"time"

	"sync/atomic"

	"github.com/pkg/errors"
)

var (
	newline = []byte("\n")
	attackLine = []byte("ATTACK: ")
)

var (
	// ErrWorkerNotFinished is returned when worker finished processing and response is not yet received.
	ErrWorkerNotFinished = errors.New("worker response not yet received")

	// ErrWorkerFinished is returned when worker is finished processing and response was received therefore not
	// allowing changes to be made to it's context.
	ErrWorkerFinished = errors.New("worker response already received")
)

// WorkerFunc defines a function type which generates a new Worker for running a load test.
type WorkerFunc func() Worker

// HitSegment details the giving rate per second and maximum allowed
// hits using available max workers to send hit requests against
// a target.
type HitSegment struct {
	// Rate sets the initial rate in requests per second.
	// (Default: 10 requests / second).
	Rate float64

	// MaxHits sets the maximum amount of hits per the rate value
	// which this segment will run.
	// (Defaults: 1000).
	MaxHits int
}

func (hs *HitSegment) init() {
	if hs.Rate <= 0 {
		hs.Rate = 10
	}

	if hs.MaxHits <= 0 {
		hs.MaxHits = 1000
	}
}

// Config provides all the standard config options. Use the Initialise method to configure with a provided Config.
type Config struct {
	// WorkerFunc is responsible for generating workers for load-testing.
	WorkerFunc WorkerFunc

	// OnNextSegment sets a function to be executed once a new hit segment has begun.
	OnNextSegment func(HitSegment, Stats)

	// SegmentedEnded sets a function to be executed once a hit segment has finished.
	OnSegmentEnd func(HitSegment,  Stats)

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

	// Metrics sets the Writer to write period stats of blaster into.
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
	
	// HitCoolOff sets the duration for which we cool off before starting the next
	// hit segments, this allows us to provide some time for the API to respond to
	// pending possible requests.
	// (Default: 1 second).
	HitCoolOff time.Duration

	// Endless sets whether the blaster should still continue working, if it has exhausted
	// it's segments list. This allows us dynamically deliver new segments to be delivered
	// into a current running blaster.
	Endless bool
}

// Blaster provides the back-end blast: a simple tool for API load testing and batch jobs. Use the New function to create a Blaster with default values.
type Blaster struct {
	config *Config

	softTimeout time.Duration
	hardTimeout time.Duration

	sgml     sync.RWMutex
	allSegment *HitSegment
	segments []HitSegment

	ctx    context.Context
	cancel context.CancelFunc

	mainChannel               chan int
	workerChannel             chan int
	currentHits int64
	completedHits int64
	errorChannel              chan error
	hitSegmentFinishedChannel chan struct{}
	workersFinishedChannel    chan struct{}
	itemFinishedChannel       chan struct{}
	addHitSegment             chan HitSegment
	signalChannel             chan os.Signal

	mainWait   *sync.WaitGroup
	workerWait *sync.WaitGroup

	errorsIgnored uint64
	metrics       *metricsDef
	err           error
}

// New creates a new Blaster with defaults.
func New() *Blaster {
	return &Blaster{
		signalChannel: make(chan os.Signal, 1),
		mainWait:      new(sync.WaitGroup),
		workerWait:    new(sync.WaitGroup),
		addHitSegment: make(chan HitSegment, 1),
		errorChannel:  make(chan error),
		mainChannel:   make(chan int),
		workerChannel: make(chan int),
		softTimeout:   time.Second,
		hardTimeout:   time.Second * 2,
	}
}

// Exit cancels any goroutines that are still processing, and closes all files.
func (b *Blaster) Exit() {
	b.cancel()
}

// Start starts the blast run without processing any config.
func (b *Blaster) Start(ctx context.Context, c Config) (Stats, error) {
	b.ctx, b.cancel = context.WithCancel(ctx)

	if err := b.initialiseConfig(c); err != nil {
		return Stats{}, err
	}

	b.setTimeout(b.config.Timeout)

	b.metrics = newMetricsDef(b.config, b.allSegment)

	b.workersFinishedChannel = make(chan struct{}, 0)
	b.hitSegmentFinishedChannel = make(chan struct{}, 0)

	err := b.start(b.ctx)
	return b.Stats(), err
}

// Stats returns a snapshot of the metrics (as is printed during interactive execution).
func (b *Blaster) Stats() Stats {
	return b.metrics.stats()
}

// initialiseConfig configures the Blaster with config options in a provided Config.
func (b *Blaster) initialiseConfig(c Config) error {
	if len(c.Segments) == 0 {
		var hit HitSegment
		hit.init()
		c.Segments = append(c.Segments, hit)
	}

	if c.Workers <= 0 {
		c.Workers = 10
	}

	if c.Timeout <= 0 {
		c.Timeout = time.Second
	}

	if c.PeriodicWrite <= 0 {
		c.PeriodicWrite = time.Second * 5
	}
	
	if c.HitCoolOff <= 0 {
		c.HitCoolOff = time.Second
	}
	
	var all HitSegment
	for _, seg := range c.Segments {
		all.Rate += seg.Rate
		all.MaxHits += seg.MaxHits
	}

	b.config = &c
	b.allSegment = &all
	b.segments = c.Segments
	return nil
}

func (b *Blaster) isEmptySegments() bool {
	b.sgml.RLock()
	defer b.sgml.RUnlock()
	return len(b.segments) == 0
}

func (b *Blaster) getCurrentSegment() (HitSegment, bool) {
	b.sgml.RLock()
	defer b.sgml.RUnlock()

	if len(b.segments) != 0 {
		return b.segments[0], true
	}
	return HitSegment{}, false
}

func (b *Blaster) start(ctx context.Context) error {
	hit, ok := b.getCurrentSegment()
	if !ok {
		return errors.New("No attached HitSegment test")
	}
	
	b.metrics.addSegment(&hit)

	b.startTickerLoop(ctx)
	b.startMainLoop(ctx)
	b.startErrorLoop(ctx)
	b.startWorkers(ctx)
	b.startStatsWriteLoop(ctx)

	// wait for cancel or finished
	select {
	case <-ctx.Done():
	case <-b.hitSegmentFinishedChannel:
	}

	b.println("Waiting for workers to finish...")
	b.workerWait.Wait()
	b.println("All workers finished.")

	// signal to log and error loop that it's tine to exit
	close(b.workersFinishedChannel)

	// if it's in endless mode then we need to close data channel.
	if b.config.Endless {
		close(b.hitSegmentFinishedChannel)
	}

	b.println("Waiting for processes to finish...")
	b.mainWait.Wait()
	b.println("All processes finished.")

	// if metric printer is set, print one more time the last stats.
	if b.config.Metrics != nil {
		b.config.Metrics.Write([]byte(b.metrics.stats().String()))
	}

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

// setTimeout sets the timeout. See Config.Timeout for more details.
func (b *Blaster) setTimeout(timeout time.Duration) {
	b.softTimeout = timeout
	b.hardTimeout = timeout + time.Second
}

// setWorker sets the worker creation function. See httpworker for a simple example.
func (b *Blaster) setWorker(wf func() Worker) {
	b.config.WorkerFunc = wf
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

//********************************************************************
// Rate Changing
//********************************************************************

// AddHitSegment adds a new hit segment if the underline blaster as not finished.
func (b *Blaster) AddHitSegment(rate float64, maxHits int) {
	b.addHitSegment <- HitSegment{Rate: rate, MaxHits: maxHits}
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
				b.println("Exiting with no error gracefully...")
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
			case <-b.hitSegmentFinishedChannel:
				// If hitSegmentFinishedChannel is closed externally (e.g. in tests), we should return.
				return
			case segment := <-b.mainChannel:
				// ensure we exit worker channel if we were closed.
				select {
				case b.workerChannel <- segment:
				case <-b.hitSegmentFinishedChannel:
				}

			}
		}
	}()
}

//********************************************************************
// println
//********************************************************************

func (b *Blaster) println(a ...interface{}) {
	if b.config.Log == nil {
		return
	}
	fmt.Fprintln(b.config.Log, a...)
}

func (b *Blaster) printf(format string, a ...interface{}) {
	if b.config.Log == nil {
		return
	}
	fmt.Fprintf(b.config.Log, format, a...)
}

//********************************************************************
// Worker Loop
//********************************************************************

func (b *Blaster) startWorkers(ctx context.Context) {
	for i := 0; i < b.config.Workers; i++ {

		w := b.config.WorkerFunc()

		if starter, ok := w.(Starter); ok {
			if err := starter.Start(ctx); err != nil {
				b.error(errors.WithStack(err))
				return
			}
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

			var ticker = time.NewTicker(time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-b.hitSegmentFinishedChannel:
					// exit gracefully
					return
				case workSegmentID := <-b.workerChannel:
					hit, _ := b.getCurrentSegment()
					if err := b.send(ctx, w, index, workSegmentID, hit); err != nil {
						// notest
						b.error(err)
						return
					}
					if b.itemFinishedChannel != nil {
						// only used in tests
						b.itemFinishedChannel <- struct{}{}
					}
					
					// increment completed count for hits.
					atomic.AddInt64(&b.completedHits, 1)
				case <-ticker.C:
					// if no work is available, just schedule next goroutine.
					runtime.Gosched()
				}
			}
		}(i)
	}
}

const contextErrorMessage = "a worker was still sending after timeout + 1 second. This indicates a bug in the worker code." +
	" Workers should immediately exit on receiving a signal from ctx.Done()"

func (b *Blaster) send(ctx context.Context, w Worker, workerID int, segmentID int, hit HitSegment) error {
	b.metrics.logStart(segmentID)
	b.metrics.logBusy(segmentID)
	b.metrics.busy.Inc(1)
	defer b.metrics.busy.Dec(1)

	// Create a child context with the selected timeout
	child, cancel := context.WithTimeout(ctx, b.softTimeout)
	defer cancel()

	var newWorkContext, err = w.Prepare(child)
	if err != nil {
		b.error(errors.WithStack(err))
		return err
	}

	if newWorkContext == nil {
		b.error(errors.New("Worker.Start returned nil WorkerContext"))
		return err
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

	newWorkContext.hitseg = hit
	newWorkContext.worker = workerID
	newWorkContext.segment = segmentID

	// Record the start time
	newWorkContext.sendStart = time.Now()

	finished := make(chan struct{})

	go func() {
		w.Send(child, newWorkContext)
		close(finished)
	}()

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
		case <-finished: // notest
			// Only continue when finished channel is closed - e.g. sending goroutine has exited.
		case <-time.After(b.hardTimeout):
			hardTimeoutExceeded = true
		}
	case <-time.After(b.hardTimeout):
		hardTimeoutExceeded = true
	}

	if hardTimeoutExceeded {
		// If we get here then the worker is not respecting the context cancellation deadline, and
		// we should exit with an error. We don't simply log this as an unsuccessful request
		// because the sending goroutine is still running and would crete a memory leak.
		b.error(errors.New(contextErrorMessage))
		return nil
	}
	
	if !newWorkContext.IsFinished() {
		newWorkContext.SetResponse("UNFINISHED", Payload{}, errors.New("failed to finish"))
	}

	// if we did not finish the request, then we must inform that this was a
	// and unfinished request.
	b.metrics.logFinish(newWorkContext.segment, newWorkContext.treePath(), time.Since(newWorkContext.sendStart), newWorkContext.Error() == nil)
	newWorkContext.buildMetric(b.metrics)

	// Call the OnEachWorker function if set.
	if b.config.OnEachRun != nil {
		b.config.OnEachRun(workerID, newWorkContext, b.metrics.stats())
	}

	if b.config.Log != nil {
		b.config.Log.Write(attackLine)
		encoder := gojay.BorrowEncoder(b.config.Log)
		if err := encoder.EncodeObject(newWorkContext); err != nil {
			b.printf("Failed to encode WorkerContext: %+s", err)
		}
		b.config.Log.Write(newline)
	}
	return nil
}

//********************************************************************
// Ticker Loop
//********************************************************************

func (b *Blaster) startStatsWriteLoop(ctx context.Context) {
	if b.config.Metrics == nil {
		return
	}

	b.mainWait.Add(1)

	go func() {
		defer b.mainWait.Done()
		defer b.println("Exiting StatsWriteLoop loop")

		var ticker = time.NewTicker(b.config.PeriodicWrite)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				b.println("Writing new metric data into writer")
				var content = b.metrics.stats().String()
				b.config.Metrics.Write([]byte(content))
				b.config.Metrics.Write(newline)
			case <-ctx.Done():
				return
			case <-b.hitSegmentFinishedChannel:
				// If hitSegmentFinishedChannel is closed externally (e.g. in tests), we should return.
				return
			}
		}
	}()
}

func (b *Blaster) startTickerLoop(ctx context.Context) {
	b.mainWait.Add(1)

	var ticker *time.Ticker

	updateTicker := func() {
		currentSegment, ok := b.getCurrentSegment()
		if !ok {
			return
		}

		// stop last ticker to ensure cleaned up resources.
		if ticker != nil {
			ticker.Stop()
		}

		ticksPerSecond := currentSegment.Rate
		ticksPerMs := ticksPerSecond / 1000.0
		ticksPerUs := ticksPerMs / 1000.0
		ticksPerNs := ticksPerUs / 1000.0
		nsPerTick := 1.0 / ticksPerNs

		ticker = time.NewTicker(time.Nanosecond * time.Duration(nsPerTick))
	}

	checkSegment := func(lastHits int) bool {
		currentSegment, ok := b.getCurrentSegment()
		if !ok {
			// signal end of segments test list if not endless.
			if !b.config.Endless {
				b.println("No more hit segments, closing segment worker")
				close(b.hitSegmentFinishedChannel)
			}

			return false
		}

		b.printf("Checking max hits at %d for segment %#v \n", lastHits, currentSegment)

		// if we match the current allowed hits for this segment with
		// the same rate, then eject segment for next, unless it's the
		// last one, so we end also.
		if currentSegment.MaxHits <= lastHits {
			b.printf("Reached max hits %d for segment %#v \n", lastHits, currentSegment)

			if len(b.segments) == 1 {
				b.sgml.Lock()
				b.segments = b.segments[:0]
				b.sgml.Unlock()

				// signal end of segments test list if not endless.
				if !b.config.Endless {
					b.println("No more hit segments, closing segment worker")
					close(b.hitSegmentFinishedChannel)
				}

				if b.config.OnSegmentEnd != nil {
					b.config.OnSegmentEnd(currentSegment, b.metrics.stats())
				}

				return true
			}
			
			// Cool off before we start the next hit segment
			time.Sleep(b.config.HitCoolOff)

			// eject last segment and update ticker.
			var newSegment HitSegment

			b.sgml.Lock()
			b.segments = b.segments[1:]
			newSegment = b.segments[0]
			b.sgml.Unlock()

			// add new segment into our metrics collector.
			b.metrics.addSegment(&newSegment)

			if b.config.OnNextSegment != nil {
				b.config.OnNextSegment(newSegment, b.metrics.stats())
			}

			// update ticker with new segment changes.
			updateTicker()
			return true
		}

		return false
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
			case <-b.hitSegmentFinishedChannel:
				return
			case hs := <-b.addHitSegment:
				b.sgml.Lock()
				b.allSegment.MaxHits += hs.MaxHits
				b.allSegment.Rate += hs.Rate
				b.segments = append(b.segments, hs)
				b.sgml.Unlock()
			}

			// if we have processed all hit segments and
			// we are currently empty, then decide if we should stop
			// or if endless, just continue
			if b.isEmptySegments() && b.config.Endless {
				b.println("No more hit segments, waiting for new ones...")
				continue
			}

			// We will only ever increment the hits only when
			// a worker was successfully a able to pick up work
			// in main loop.
			var completedCount = int(atomic.LoadInt64(&b.completedHits))
			if checkSegment(completedCount) {
				b.println("Resetting hit count for new segment")
				atomic.StoreInt64(&b.completedHits, 0)
				atomic.StoreInt64(&b.currentHits, 0)
				continue
			}
			
			var currentSegment, ok = b.getCurrentSegment()
			if !ok {
				// notest
				select {
					case <-ctx.Done():
						return
					case <-b.hitSegmentFinishedChannel:
						// notest
						return
					default:
						continue
				}
			}
			
			// If we have reached here and maximum sent segment is
			// exactly the same as HitSegments.MaxHits, then workers
			// are not done yet, so we retry at another tick, till
			// either workers are done or the context timeout for them
			// is reached.
			var sentCount = int(atomic.LoadInt64(&b.currentHits))
			if sentCount == currentSegment.MaxHits {
				continue
			}
			
			var segment = b.metrics.currentSegment()

			// Next send on the main channel. The channel won't have a listener if there is no idle
			// worker. In this case we should continue and log a miss.
			select {
			case <-ctx.Done():
				// notest
				return
			case <-b.hitSegmentFinishedChannel:
				// notest
				return
			case b.mainChannel <- segment:
				var currentCount = atomic.LoadInt64(&b.currentHits)
				b.printf("Sending segment %d for processing at hit %d \n", segment, currentCount)
				atomic.AddInt64(&b.currentHits, 1)
				continue
			default:
				// if main loop is busy, skip this tick
				// and retry again.
				continue
			}
		}
	}()
}
