package tests

import (
	"bytes"
	"context"
	"fmt"
	actservice "github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/internal/ActService_listen_file"
	"github.com/golang/glog"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func newTestListener(t *testing.T, ls *ActService_listen_file.ListenerSet) (*ActService_listen_file.InProgressListener, chan bool) {
	endIter := make(chan bool, 1)
	yield := make(chan *actservice.JobLogMessage, 1)
	ctx, cancel := context.WithCancel(context.Background())

	endCause := ActService_listen_file.EndIterCause{
		EndOnEOF: false,
		EndIter:  endIter,
	}
	listener := ActService_listen_file.NewInProgressListener(&endCause, cancel)
	finalizer, err := ls.AddListener(listener)
	finalized := make(chan bool, 1)
	finalizerWrapper := func() {
		finalizer()
		finalized <- true
		glog.V(1).Info("finalizer called")
	}
	require.NoError(t, err)
	// will be blocked
	emptyReader := bytes.NewReader([]byte{})

	go func() {
		err := ActService_listen_file.ListenFile(
			ctx,
			emptyReader,
			0,
			&endCause,
			yield,
			finalizerWrapper,
		)
		require.NoError(t, err)
	}()

	return listener, finalized
}

func TestAddListener(t *testing.T) {
	ls := ActService_listen_file.NewListenerSet()
	listener, finalized := newTestListener(t, &ls)
	listener.ForceCancel()
	select {
	case <-finalized:
		glog.V(1).Info("Listener was finalized")
		break
	case <-time.After(time.Second * 10):
		require.Fail(t, "Listener was not finalized")
	}
}

func checkFinalizers(t *testing.T, channels []chan bool) {
	for idx, channel := range channels {
		select {
		case <-channel:
			glog.V(1).Info("Listener was finalized")
			break
		case <-time.After(time.Second * 5):
			require.Fail(t, fmt.Sprintf("Listener %d was not finalized", idx))
		}
	}
}

func TestCancelEach_SendsSignal(t *testing.T) {
	ls := ActService_listen_file.NewListenerSet()

	_, finalized1 := newTestListener(t, &ls)
	_, finalized2 := newTestListener(t, &ls)

	err := ls.CancelEach()
	require.NoError(t, err)

	checkFinalizers(t, []chan bool{finalized1, finalized2})
}

func TestSetExitOnEof(t *testing.T) {
	ls := ActService_listen_file.NewListenerSet()

	_, finalized1 := newTestListener(t, &ls)
	_, finalized2 := newTestListener(t, &ls)

	ls.SetExitOnEof()
	checkFinalizers(t, []chan bool{finalized1, finalized2})
}
