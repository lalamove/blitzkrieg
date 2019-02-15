package blitzkrieg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/francoispqt/gojay"
)

// Stringify returns a giving value as a string.
func Stringify(v interface{}) string {
	switch v := v.(type) {
	case string:
		return v
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr, float32, float64, complex64, complex128:
		return fmt.Sprint(v)
	default:
		j, _ := json.Marshal(v)
		return string(j)
	}
}

//********************************************************************
// Worker
//********************************************************************

// Worker is an interface that allows blast to easily be extended to support any protocol.
// A worker receives a WorkerContext which holds underline data about a request to be made
// and can be provided with details for it's response.
//
// The WorkerContext provides the WorkerContext.FromContext which creates a parent-child chain
// for a context, this allows a single worker to make multiple request each with a unique title
// that the parent will collect, this tree will be followed to generate a complete view and
// metric information on the whole and individual requests for the worker.
//
// By using this architecture we allow workers which are created once but called
// repeatedly to process requests concurrently and house different internal behaviours or
// sub calls which we can equally measure without much complexity in the library itself.
type Worker interface {
	// Prepare handles the loading of the initial data a worker should begin with, it is
	// responsible for creating what data a request should be or contain.
	Prepare(ctx context.Context) (*WorkerContext, error)

	// Send handles the internal request/requests we wish to make for to our title.
	//
	// You only make request for a single or set of requests for just one call to your
	// service title. Blitzkrieg handles concurrent hits to your title by calling
	// multiple versions of your worker.
	Send(ctx context.Context, workerCtx *WorkerContext)
}

// Starter is an interface a worker can optionally satisfy to provide initialization logic.
type Starter interface {
	Start(ctx context.Context) error
}

// Stopper is an interface a worker can optionally satisfy to provide finalization logic.
type Stopper interface {
	Stop(ctx context.Context) error
}

//********************************************************************
// FunctionWorker
//********************************************************************

// FunctionWorker facilitates code examples by satisfying the Worker, Starter and Stopper interfaces with provided functions.
type FunctionWorker struct {
	StopFunc    func(ctx context.Context) error
	StartFunc   func(ctx context.Context) error
	PrepareFunc func(ctx context.Context) (*WorkerContext, error)
	SendFunc    func(ctx context.Context, workerCtx *WorkerContext)
}

// Send satisfies the Worker interface.
func (e *FunctionWorker) Send(ctx context.Context, lastWctx *WorkerContext) {
	if e.SendFunc == nil {
		return
	}
	e.SendFunc(ctx, lastWctx)
}

// Prepare satisfies the Worker interface.
func (e *FunctionWorker) Prepare(ctx context.Context) (*WorkerContext, error) {
	if e.PrepareFunc == nil {
		return WorkerContextWithoutPayload(nil), nil
	}
	return e.PrepareFunc(ctx)
}

// Start satisfies the Starter interface.
func (e *FunctionWorker) Start(ctx context.Context) error {
	if e.StartFunc == nil {
		return nil
	}
	return e.StartFunc(ctx)
}

// Stop satisfies the Stopper interface.
func (e *FunctionWorker) Stop(ctx context.Context) error {
	if e.StopFunc == nil {
		return nil
	}
	return e.StopFunc(ctx)
}

//********************************************************************
// Worker Context
//********************************************************************

// Payload is defines the content to be used for a giving request with it's headers
// body and possible parameters depending on the underline protocol logic.
type Payload struct {
	Body    []byte
	Params  map[string]string
	Headers map[string][]string
}

// IsNil implements gojay.MarshalJSONObject interface method.
func (p Payload) IsNil() bool {
	return false
}

// MarshalJSONObject implements gojay.MarshalJSONObject interface.
func (p Payload) MarshalJSONObject(encoder *gojay.Encoder) {
	encoder.StringKey("body", string(p.Body))
	encoder.ObjectKey("params", paramEncodable(p.Params))
	encoder.ObjectKey("headers", headersEncodable(p.Headers))
}

type paramEncodable map[string]string

// IsNil implements gojay.MarshalJSONObject interface method.
func (p paramEncodable) IsNil() bool {
	return false
}

// MarshalJSONObject implements gojay.MarshalJSONObject interface.
func (p paramEncodable) MarshalJSONObject(enc *gojay.Encoder) {
	for key, value := range p {
		enc.AddStringKey(key, value)
	}
}

type stringListEncoding []string

// IsNil implements gojay.MarshalJSONObject interface method.
func (p stringListEncoding) IsNil() bool {
	return false
}

// MarshalJSONObject implements gojay.MarshalJSONObject interface.
func (p stringListEncoding) MarshalJSONArray(enc *gojay.Encoder) {
	for _, value := range p {
		enc.AddString(value)
	}
}

type headersEncodable map[string][]string

// IsNil implements gojay.MarshalJSONObject interface method.
func (p headersEncodable) IsNil() bool {
	return false
}

// MarshalJSONObject implements gojay.MarshalJSONObject interface method.
func (p headersEncodable) MarshalJSONObject(enc *gojay.Encoder) {
	for key, value := range p {
		enc.AddArrayKey(key, stringListEncoding(value))
	}
}

// WorkerContext exists to define and contain the request body, headers and
// response content, header and status for a giving request work done by
// a worker. It also provides a means of providing response from a previous
// request to a next request in a sequence or for the desire of alternating
// the behaviour of the next worker based on the response from the last.
//
// WorkerContext is not safe for concurrent use by multiple go-routines, nor is it
// intended to be.
type WorkerContext struct {
	segment     int
	worker      int
	hitseg      HitSegment
	title       string
	segmentID   string
	meta        interface{}
	end         time.Time
	start       time.Time
	sendStart   time.Time
	requestBody Payload

	workerErr       error
	workerStatus    string
	responseBody    Payload
	finishedRequest bool

	parent   *WorkerContext
	children []*WorkerContext
}

// NewWorkerContext returns a new WorkerContext which has no previous context.
// Arguments:
//	 - meta: This is any meta type you wish to be attached to your WorkerContext
//          like a Config for the title information.
func NewWorkerContext(title string, req Payload, meta interface{}) *WorkerContext {
	return requestWorkerContext(title, req, meta, nil)
}

// WorkerContextWithoutPayload returns a new WorkerContext with a default empty
// payload.
//
// Always provide a descriptive title for a worker context for easy recognition.
func WorkerContextWithoutPayload(meta interface{}) *WorkerContext {
	return requestWorkerContext("(root)", Payload{}, meta, nil)
}

// FromContext returns a new WorkerContext which is based of this worker
// context, connecting this as it's previous context.
//
// Always provide a descriptive title for a worker context for easy recognition.
func (w *WorkerContext) FromContext(title string, nextReq Payload, meta interface{}) *WorkerContext {
	return requestWorkerContext(title, nextReq, meta, w)
}

// requestWorkerContext returns a new WorkerContext which has giving request
// payload and last context for use with title set as desired.
//
// Always provide a descriptive title for a worker context for easy recognition.
func requestWorkerContext(title string, req Payload, meta interface{}, last *WorkerContext) *WorkerContext {
	var w = new(WorkerContext)
	w.meta = meta
	w.parent = last
	w.title = title
	w.segmentID = title
	w.requestBody = req
	w.start = time.Now()

	if last != nil {
		w.sendStart = w.start
		w.segmentID = fmt.Sprintf("%s/%s", last.segmentID, title)
		last.children = append(last.children, w)
		w.segment = last.segment
	}
	return w
}

// ParentContext returns parent worker context from where it
// is derived from. A root WorkerContext never has a parent.
func (w *WorkerContext) LastContext() *WorkerContext {
	return w.parent
}

// Error returns occured error for last executed worker.
func (w *WorkerContext) Error() error {
	return w.workerErr
}

// Request returns Payload for giving request context.
func (w *WorkerContext) Request() Payload {
	return w.requestBody
}

// Meta returns attached Meta if any for giving request context.
func (w *WorkerContext) Meta() interface{} {
	return w.meta
}

// IsFinished returns true/false if giving context is finished and
// concluded.
func (w *WorkerContext) IsFinished() bool {
	return w.finishedRequest
}

// Status returns response status for giving request worker
// and is not available until after a request was finished.
func (w *WorkerContext) Status() string {
	return w.workerStatus
}

// Elapsed returns the total duration taking from the creation of a context
// till the call to it's method SetResponse().
func (w *WorkerContext) Elapsed() time.Duration {
	return w.end.Sub(w.start)
}

// Since returns the total duration taking from the passed in time of a context
// till the call to it's method SetResponse().
func (w *WorkerContext) Since(from time.Time) time.Duration {
	return from.Sub(w.end)
}

// IsNil implements gojay.MarshalJSONObject interface method.
func (w *WorkerContext) IsNil() bool {
	return false
}

func (w *WorkerContext) treePath() string {
	if w.finishedRequest {
		return fmt.Sprintf("%s (%s)", w.segmentID, w.workerStatus)
	}
	return fmt.Sprintf("%s (UNFINISHED)", w.segmentID)
}

func (w *WorkerContext) buildMetric(root *metricsDef) {
	for _, child := range w.children {
		root.logSegmentFinish(child.segment, child.treePath(), time.Since(child.sendStart), child.workerErr == nil)
		child.buildMetric(root)
	}
}

// MarshalJSONObject implements gojay.MarshalJSONObject interface method.
func (w *WorkerContext) MarshalJSONObject(enc *gojay.Encoder) {
	enc.IntKey("segment", w.segment)
	enc.IntKey("worker_int", w.worker)
	enc.StringKey("segment_id", w.segmentID)
	enc.BoolKey("finished", w.finishedRequest)
	enc.StringKey("request_target", w.title)
	enc.StringKey("response_status", w.workerStatus)
	enc.ObjectKey("request_payload", w.requestBody)

	if w.finishedRequest {
		enc.ObjectKey("response_body", w.responseBody)
	}

	if w.workerErr != nil {
		enc.StringKey("response_error", w.workerErr.Error())
	}
}

// Response returns Payload for giving response, this is only ever available
// to the next sequence once the current has completed it's run in a
// set of sequence request.
func (w *WorkerContext) Response() (Payload, error) {
	if w.finishedRequest {
		return w.responseBody, nil
	}
	return w.responseBody, ErrWorkerNotFinished
}

// SetResponse sets the response of the worker context, finished it and setting
// the response payload and status and possible error which you wish to set as
// the signifying error for giving request else this get's set to the returned error
// from a worker when finished a giving run.
//
// if an error occurs on the worker or if the response err value is set then we consider
// the work a failure and not a success.
func (w *WorkerContext) SetResponse(status string, payload Payload, err error) error {
	if w.finishedRequest {
		return ErrWorkerFinished
	}

	w.workerErr = err
	w.end = time.Now()
	w.workerStatus = status
	w.responseBody = payload
	w.finishedRequest = true
	return nil
}
