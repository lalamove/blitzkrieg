package blitzkrieg_test

import (
	"context"
	"github.com/lalamove/blitzkrieg"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPayload(t *testing.T){
	var p blitzkrieg.Payload
	p.Body = []byte("No")
	
	p.AddParam("param", "2")
	require.Equal(t, p.Params["param"], "2")
	
	p.AddParam("litter", "1")
	require.Equal(t, p.Params["litter"], "1")
	
	p.AddHeader("content", "1", "2", "3")
	require.Contains(t, p.Headers["content"], "1")
	require.Contains(t, p.Headers["content"], "2")
	require.Contains(t, p.Headers["content"], "3")
	
	var newPayload = p.From([]byte("yes"))
	require.NotEqual(t, p, newPayload)
	require.NotEqual(t, p.Body, newPayload.Body)
	require.Contains(t, newPayload.Headers["content"], "1")
	require.Contains(t, newPayload.Headers["content"], "2")
	require.Contains(t, newPayload.Headers["content"], "3")
	
	var withPayload = p.With(map[string]string{"gritter":"40"}, nil, nil)
	require.NotEqual(t, p, withPayload)
	require.Equal(t, p.Body, withPayload.Body)
	require.Contains(t, withPayload.Headers["content"], "1")
	require.Contains(t, withPayload.Headers["content"], "2")
	require.Contains(t, withPayload.Headers["content"], "3")
	
}

func TestDefaultFunctionWorker(t *testing.T){
	var w blitzkrieg.FunctionWorker
	
	require.NoError(t, w.Start(context.Background()))
	ctx, err := w.Prepare(context.Background())
	require.NoError(t, err)
	require.NotNil(t, ctx)
	
	w.Send(context.Background(), ctx)
	
	require.NoError(t, w.Stop(context.Background()))
}

func TestFunctionWorkerWithValues(t *testing.T){
	var w blitzkrieg.FunctionWorker
	w.PrepareFunc = func(ctx context.Context) (workerContext *blitzkrieg.WorkerContext, e error) {
		return blitzkrieg.NewWorkerContext("no-context", blitzkrieg.Payload{}, nil), nil
	}
	w.SendFunc = func(ctx context.Context, workerCtx *blitzkrieg.WorkerContext) {
		workerCtx.SetResponse("200", blitzkrieg.Payload{}, nil)
	}
	w.StartFunc = func(ctx context.Context) error {
		return nil
	}
	w.StopFunc = func(ctx context.Context) error {
		return nil
	}
	
	require.NoError(t, w.Start(context.Background()))
	ctx, err := w.Prepare(context.Background())
	require.NoError(t, err)
	require.NotNil(t, ctx)
	
	w.Send(context.Background(), ctx)
	require.NoError(t, w.Stop(context.Background()))
	
	require.Equal(t, "200", ctx.Status())
	require.True(t, ctx.IsFinished())
}
