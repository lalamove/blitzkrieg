package blast

import (
	"context"
	"fmt"
	"github.com/francoispqt/gojay"

	"encoding/json"
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
// and a possible response, you are allowed to return the same or a new WorkerContext for
// the purpose of presenting a connected set of multiple WorkerContext when your worker
// is a combination of multiple requests which are part of a whole.
//
// This allows workers which are created once but called repeatedly to process requests
// concurrently to house the whole behaviour for the load test without complexity
// loaded into this library itself, so you can flexibly create a worker which is a
// sequence of request but still be able to provide base detail when a worker is done.
type Worker interface {
	Send(ctx context.Context, workerCtx *WorkerContext) (*WorkerContext, error)
}

// Starter is an interfaces a worker can optionally satisfy to provide initialization
// logic.
//
// The starter handles the loading of the initial data a worker should begin with, it is
// responsible for creating what data a request should be or contain.
type Starter interface {
	Start(ctx context.Context) (*WorkerContext, error)
}

// Stopper is an interface a worker can optionally satisfy to provide finalization logic.
type Stopper interface {
	Stop(ctx context.Context) error
}

//********************************************************************
// Worker Context
//********************************************************************

// Payload is defines the content to be used for a giving request with it's headers
// body and possible parameters depending on the underline protocol logic.
type Payload struct{
	Body []byte
	Params map[string]string
	Headers map[string][]string
}

// IsNil implements gojay.MarshalJSONObject interface method.
func (p Payload) IsNil() bool {
	return false
}

// MarshalJSONObject implements gojay.MarshalJSONObject interface.
func (p Payload) MarshalJSONObject(encoder *gojay.Encoder){
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
// WorkerContext are not safe for concurrent writes by multiple go-routines.
// But can be accessed within multiple go-routines.
type WorkerContext struct{
	target string
	requestBody Payload
	lastWorker *WorkerContext

	workerErr error
	workerStatus string
	responseBody Payload
	finishedRequest bool
}

// NewWorkerContext returns a new WorkerContext which has no previous context.
func NewWorkerContext(target string, req Payload) *WorkerContext {
	return requestWorkerContext(target, req, nil)
}

// WorkerContextWithoutPayload returns a new WorkerContext with a default empty
// payload.
func WorkerContextWithoutPayload() *WorkerContext {
	return requestWorkerContext("", Payload{}, nil)
}

// requestWorkerContext returns a new WorkerContext which has giving request
// payload and last context for use.
func requestWorkerContext(target string, req Payload, last *WorkerContext) *WorkerContext {
	return &WorkerContext{
		target: target,
		requestBody: req,
		lastWorker: last,
	}
}

// FromContext returns a new WorkerContext which is based of this worker
// context, connecting this as it's previous context.
func (w *WorkerContext) FromContext(target string, nextReq Payload) *WorkerContext {
	return requestWorkerContext(target, nextReq, w)
}

// LastContext returns last request-request context for last execution
// in a set sequence of request if any, else returning nil.
func (w *WorkerContext) LastContext() *WorkerContext {
	return w.lastWorker
}

// Error returns occured error for last executed worker.
func (w *WorkerContext) Error() error {
	return w.workerErr
}

// Request returns Payload for giving request context.
func (w *WorkerContext) Request() Payload {
	return w.requestBody
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

// IsNil implements gojay.MarshalJSONObject interface method.
func (w *WorkerContext) IsNil() bool {
	return false
}

// MarshalJSONObject implements gojay.MarshalJSONObject interface method.
func (w *WorkerContext) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("request_target", w.target)
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
func (w *WorkerContext) SetResponse(status string, payload Payload, err error) error {
	if w.finishedRequest {
		return ErrWorkerFinished
	}

	w.workerErr = err
	w.workerStatus = status
	w.responseBody = payload
	w.finishedRequest = true
	return nil
}


//********************************************************************
// FunctionWorker
//********************************************************************

// FunctionWorker facilitates code examples by satisfying the Worker, Starter and Stopper interfaces with provided functions.
type FunctionWorker struct {
	StopFunc  func(ctx context.Context) error
	StartFunc func(ctx context.Context) (*WorkerContext, error)
	SendFunc  func(ctx context.Context,  lastWctx *WorkerContext) (*WorkerContext, error)
}

// Send satisfies the Worker interface.
func (e *FunctionWorker) Send(ctx context.Context,  lastWctx *WorkerContext) (*WorkerContext, error) {
	if e.SendFunc == nil {
		return lastWctx, nil
	}
	return e.SendFunc(ctx, lastWctx)
}

// Start satisfies the Starter interface.
func (e *FunctionWorker) Start(ctx context.Context) (*WorkerContext, error) {
	if e.StartFunc == nil {
		return WorkerContextWithoutPayload(), nil
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
