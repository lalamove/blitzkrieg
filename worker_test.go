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
