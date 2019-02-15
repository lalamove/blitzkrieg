package blitzkrieg_test

import (
	"context"
	"github.com/lalamove/blitzkrieg"
	"github.com/stretchr/testify/require"
	"testing"
)

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
