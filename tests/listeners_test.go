package tests

import (
	"bytes"
	"context"
	"github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/internal/ActService_listen_file"
	"github.com/D1-3105/ActService/internal/ActService_listen_job"
	"github.com/D1-3105/ActService/pkg/actCmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func fixtureGenOutput(t *testing.T) *bytes.Buffer {
	writeBuf := bytes.NewBuffer([]byte{})
	dummy := SuccessfulDummyJobOutput()
	ctxJob, cancelJob := context.WithCancel(context.Background())
	finalizedJob := false

	go ActService_listen_job.ListenJob(
		ctxJob,
		actCmd.CommandOutput(dummy),
		writeBuf,
		"random-string",
		func() {
			defer dummy.Close()
			finalizedJob = true
			cancelJob()
		},
	)

	go DummyEmulator(ctxJob, dummy)

	select {
	case <-ctxJob.Done():
		require.True(t, finalizedJob, "job should have finalized")
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for ListenJob to complete")
	}
	return writeBuf
}

func TestListenFile(t *testing.T) {
	writeBuf := fixtureGenOutput(t)

	// ListenFile block
	ctxRead, cancelRead := context.WithCancel(context.Background())
	defer cancelRead()

	yieldChan := make(chan *actservice.JobLogMessage, 10)
	finalizedFile := false
	endCause := &ActService_listen_file.EndIterCause{
		EndOnEOF: true,
		EndIter:  make(chan bool, 1),
	}

	go func() {
		err := ActService_listen_file.ListenFile(
			ctxRead,
			bytes.NewReader(writeBuf.Bytes()),
			0, // readOffset
			endCause,
			yieldChan,
			func() {
				finalizedFile = true
			},
		)
		require.NoError(t, err)
	}()
	var messages []*actservice.JobLogMessage
readLoop:
	for {
		select {
		case msg, ok := <-yieldChan:
			if !ok {
				break readLoop
			}
			messages = append(messages, msg)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout reading from yieldChan")
		}
	}

	require.True(t, finalizedFile, "file reader should have finalized")
	require.NotEmpty(t, messages, "should have read at least one message")
	for _, msg := range messages {
		assert.NotEmpty(t, msg.Line)
		assert.True(t, msg.Timestamp > 0)
	}
}

func TestListenFile_EmptyFile_EndOnEOFTrue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	yieldChan := make(chan *actservice.JobLogMessage, 1)
	endCause := &ActService_listen_file.EndIterCause{
		EndOnEOF: true,
		EndIter:  make(chan bool, 1),
	}

	err := ActService_listen_file.ListenFile(
		ctx,
		bytes.NewReader([]byte{}),
		0,
		endCause,
		yieldChan,
		func() {},
	)

	require.NoError(t, err)
	_, ok := <-yieldChan
	require.False(t, ok, "yieldChan should be closed")
}

func TestListenFile_ReadOffset_SkipFirstTwo(t *testing.T) {
	buf := fixtureGenOutput(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	yieldChan := make(chan *actservice.JobLogMessage, 10)
	endCause := &ActService_listen_file.EndIterCause{
		EndOnEOF: true,
		EndIter:  make(chan bool, 1),
	}

	finalized := false
	go func() {
		err := ActService_listen_file.ListenFile(ctx, bytes.NewReader(buf.Bytes()), 2, endCause, yieldChan, func() {
			finalized = true
		})
		require.NoError(t, err)
	}()

	var results []*actservice.JobLogMessage
	for msg := range yieldChan {
		results = append(results, msg)
	}

	require.Len(t, results, 8)
	require.True(t, finalized)
}
