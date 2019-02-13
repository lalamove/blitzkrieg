package blitzkrieg_test

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gokit/blitzkrieg"
)

//*************************************
// Single Sequence Worker
//*************************************

type singleSequenceWorker struct {
	PrepareCalls int
	SendCalls    int
	Timeout      time.Duration
}

func (s *singleSequenceWorker) Prepare(ctx context.Context) (*blitzkrieg.WorkerContext, error) {
	s.PrepareCalls++
	return blitzkrieg.NewWorkerContext("SleepWorker", blitzkrieg.Payload{}, nil), nil
}

func (s *singleSequenceWorker) Send(ctx context.Context, workerCtx *blitzkrieg.WorkerContext) error {
	s.SendCalls++
	time.Sleep(s.Timeout)
	workerCtx.SetResponse("200", blitzkrieg.Payload{
		Body:   []byte("Ready!"),
		Params: map[string]string{"id": "1"},
	}, nil)
	return nil
}

func TestSingleWorkerWorker(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	var logBuffer bytes.Buffer
	var logWriter = NewWriteCounter(&logBuffer)

	var metricBuffer bytes.Buffer
	var metricWriter = NewWriteCounter(&metricBuffer)

	var statChan = make(chan blitzkrieg.Stats, 1)
	var errChan = make(chan error, 1)

	blits := blitzkrieg.New()
	go func() {
		defer wg.Done()

		stats, err := blits.Start(context.Background(), blitzkrieg.Config{
			Log:           logWriter,
			Metrics:       metricWriter,
			PeriodicWrite: time.Millisecond * 600,
			OnSegmentEnd:  func(segment blitzkrieg.HitSegment) {},
			OnNextSegment: func(segment blitzkrieg.HitSegment) {},
			Segments: []blitzkrieg.HitSegment{
				{
					Rate:    10,
					MaxHits: 10,
				},
				{
					Rate:    10,
					MaxHits: 10,
				},
			},
			WorkerFunc: func() blitzkrieg.Worker {
				return &singleSequenceWorker{
					Timeout: time.Millisecond * 300,
				}
			},
		})

		errChan <- err
		statChan <- stats
	}()

	wg.Wait()

	require.NoError(t, <-errChan)

	stat := <-statChan
	require.NotEmpty(t, stat)
	require.NotEmpty(t, logBuffer)
	require.NotEmpty(t, metricBuffer)

}

func TestFunctionWorker(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	blits := blitzkrieg.New()
	go func() {
		defer wg.Done()

		_, err := blits.Start(context.Background(), blitzkrieg.Config{
			Segments: []blitzkrieg.HitSegment{
				{
					Rate:    10,
					MaxHits: 10,
				},
				{
					Rate:    10,
					MaxHits: 10,
				},
			},
			WorkerFunc: func() blitzkrieg.Worker {
				return &blitzkrieg.FunctionWorker{
					SendFunc: func(ctx context.Context, lastWctx *blitzkrieg.WorkerContext) error {
						require.Nil(t, lastWctx.Meta())
						require.NoError(t, lastWctx.Error())
						require.Empty(t, lastWctx.Status())

						payload, err := lastWctx.Response()
						require.Error(t, err)
						require.Empty(t, payload)

						require.False(t, lastWctx.IsFinished())
						require.NotNil(t, lastWctx.Request())

						return lastWctx.SetResponse("200", blitzkrieg.Payload{}, nil)
					},
				}
			},
		})

		require.NoError(t, err)
	}()

	wg.Wait()
}

//*************************************
// Multi Sequence Worker
//*************************************

type multiSequenceWorker struct {
	Timeout time.Duration
}

func (s *multiSequenceWorker) Prepare(ctx context.Context) (*blitzkrieg.WorkerContext, error) {
	return blitzkrieg.NewWorkerContext("multi-sequence-worker", blitzkrieg.Payload{}, nil), nil
}

func (s *multiSequenceWorker) Send(ctx context.Context, workerCtx *blitzkrieg.WorkerContext) error {

	// initiate first sequence in multi-sequence requests
	if err := s.doFirstSequence(ctx, workerCtx.FromContext("first-sequence", blitzkrieg.Payload{}, nil)); err != nil {
		return err
	}

	// initiate second sequence in multi-sequence requests
	if err := s.doSecondSequence(ctx, workerCtx.FromContext("second-sequence", blitzkrieg.Payload{}, nil)); err != nil {
		return err
	}

	time.Sleep(s.Timeout)

	// send final response.
	workerCtx.SetResponse("200", blitzkrieg.Payload{
		Body:   []byte("Ready!"),
		Params: map[string]string{"id": "1"},
	}, nil)
	return nil
}

func (s *multiSequenceWorker) doSecondSequence(ctx context.Context, workerCtx *blitzkrieg.WorkerContext) error {
	return s.doSequence(ctx, []byte("second-sequence-data"), workerCtx)
}

func (s *multiSequenceWorker) doFirstSequence(ctx context.Context, workerCtx *blitzkrieg.WorkerContext) error {
	return s.doSequence(ctx, []byte("first-sequence-data"), workerCtx)
}

func (s *multiSequenceWorker) doSequence(ctx context.Context, data []byte, workerCtx *blitzkrieg.WorkerContext) error {
	time.Sleep(s.Timeout)

	workerCtx.SetResponse("200", blitzkrieg.Payload{
		Body:   data,
		Params: map[string]string{"id": "1"},
	}, nil)
	return nil
}

func TestMultiSequenceWorker(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	var logBuffer bytes.Buffer
	var logWriter = NewWriteCounter(&logBuffer)

	var metricBuffer bytes.Buffer
	var metricWriter = NewWriteCounter(&metricBuffer)

	var statChan = make(chan blitzkrieg.Stats, 1)
	var errChan = make(chan error, 1)

	blits := blitzkrieg.New()
	go func() {
		defer wg.Done()

		stats, err := blits.Start(context.Background(), blitzkrieg.Config{
			Log:           logWriter,
			Metrics:       metricWriter,
			PeriodicWrite: time.Millisecond * 60,
			OnSegmentEnd:  func(segment blitzkrieg.HitSegment) {},
			OnNextSegment: func(segment blitzkrieg.HitSegment) {},
			Segments: []blitzkrieg.HitSegment{
				{
					Rate:    10,
					MaxHits: 10,
				},
				{
					Rate:    10,
					MaxHits: 10,
				},
			},
			WorkerFunc: func() blitzkrieg.Worker {
				return &multiSequenceWorker{
					Timeout: time.Millisecond * 300,
				}
			},
		})

		errChan <- err
		statChan <- stats
	}()

	wg.Wait()

	require.NoError(t, <-errChan)

	stat := <-statChan
	require.NotEmpty(t, stat)
	require.NotEmpty(t, logBuffer)
	require.NotEmpty(t, metricBuffer)
}

//************************************************
// others
//************************************************

type hitWorker struct {
	hits int64
}

func (s *hitWorker) Prepare(ctx context.Context) (*blitzkrieg.WorkerContext, error) {
	return blitzkrieg.NewWorkerContext("hit-worker", blitzkrieg.Payload{}, nil), nil
}

func (s *hitWorker) Send(ctx context.Context, workerCtx *blitzkrieg.WorkerContext) error {
	atomic.AddInt64(&s.hits, 1)
	return nil
}

func TestBlasterExitSignal(t *testing.T) {
	var errChan = make(chan error, 1)
	var signalChan = make(chan struct{}, 1)
	var done = make(chan struct{}, 0)

	var hit = new(hitWorker)
	blits := blitzkrieg.New()
	go func() {
		_, err := blits.Start(context.Background(), blitzkrieg.Config{
			Workers: 1,
			Endless: true,
			OnSegmentEnd: func(_ blitzkrieg.HitSegment) {
				done <- struct{}{}
			},
			Segments: []blitzkrieg.HitSegment{
				{
					Rate:    10,
					MaxHits: 10,
				},
			},
			WorkerFunc: func() blitzkrieg.Worker {
				return hit
			},
		})

		errChan <- err
		signalChan <- struct{}{}
	}()

	<-done

	select {
	case <-errChan:
		require.Fail(t, "Should not have received error")
	case <-signalChan:
		require.Fail(t, "Should not have received end signal")
	case <-time.After(2 * time.Second):
		require.Equal(t, 10, int(hit.hits))
	}

	// exit since it's running in endless mode.
	blits.Exit()

	select {
	case <-signalChan:
		require.Len(t, errChan, 1)
		require.Nil(t, <-errChan)
	case <-time.After(1 * time.Second):
		require.Equal(t, 10, int(hit.hits))
	}
}

func TestBlasterAddHitSegmentAfterFinishedSegment(t *testing.T) {
	var r sync.WaitGroup
	r.Add(1)

	var w sync.WaitGroup
	w.Add(1)

	var hit = new(hitWorker)
	blits := blitzkrieg.New()

	var errChan = make(chan error, 1)
	go func() {
		defer r.Done()

		_, err := blits.Start(context.Background(), blitzkrieg.Config{
			Workers: 2,
			Endless: true,
			OnSegmentEnd: func(_ blitzkrieg.HitSegment) {
				w.Done()
			},
			Segments: []blitzkrieg.HitSegment{
				{
					Rate:    10,
					MaxHits: 10,
				},
			},
			WorkerFunc: func() blitzkrieg.Worker {
				return hit
			},
		})

		errChan <- err
	}()

	w.Wait()

	w.Add(1)

	blits.AddHitSegment(10, 10)

	w.Wait()

	// exit since it's running in endless mode.
	blits.Exit()

	r.Wait()
	require.Equal(t, 20, int(hit.hits))
	require.Len(t, errChan, 1)
	require.Nil(t, <-errChan)
}
