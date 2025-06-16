package ActService_listen_file

import (
	"context"
	"errors"
	"github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/internal/proto_utils"
	"github.com/golang/glog"
	"io"
)

type EndIterCause struct {
	EndIter  chan bool
	EndOnEOF bool
}

func ListenFile(
	ctx context.Context,
	jobFile io.Reader,
	readOffset uint64,
	endIterCause *EndIterCause,
	yieldChan chan *actservice.JobLogMessage,
	finalizer func(),
) error {
	defer finalizer()
	defer close(yieldChan)
	defer close(endIterCause.EndIter)
	curOffset := 0

	die := make(chan bool, 3)
	eofIo := make(chan bool, 1)
	errorChan := make(chan error, 1)

	readerCtx, readerCancel := context.WithCancel(context.Background())
	defer readerCancel()
	// Watcher goroutine
	go func(ctx context.Context) {
		defer readerCancel()
		for {
			select {
			case <-ctx.Done():
				die <- true
				return
			case end := <-endIterCause.EndIter:
				glog.V(2).Infof("Received EndIter: %v for writer %v", end, jobFile)
				die <- end
				return
			case eof := <-eofIo:
				shallDie := eof && endIterCause.EndOnEOF
				select {
				case die <- shallDie:
					if shallDie {
						glog.V(2).Infof(
							"shallDie %t=[%t and %t] sent to bound channel for writer %v",
							shallDie, eof, endIterCause.EndOnEOF, jobFile,
						)
					}

					break
				case <-ctx.Done():
					break
				}
				if shallDie {
					return
				}
			}
		}
	}(ctx)
	everyX := 5
	// Reader goroutine
	go func(ctx context.Context) {
		var blackHole actservice.JobLogMessage
		// Skip up to readOffset
		for curOffset < int(readOffset) {
			err := proto_utils.Read(jobFile, &blackHole)
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				select {
				case eofIo <- true:
				case <-ctx.Done():
					return
				}
				// wait for decision
				select {
				case shallDie := <-die:
					if shallDie {
						return
					}
				case <-ctx.Done():
					return
				}
				continue
			}
			if err != nil {
				select {
				case errorChan <- err:
				case <-ctx.Done():
				}
				return
			}
			curOffset++
			if curOffset%everyX == 0 {
				glog.V(2).Infof("Skipped line num %d", curOffset)
			}
		}

		// Actual reading loop
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var logMessage actservice.JobLogMessage
				err := proto_utils.Read(jobFile, &logMessage)
				if err != nil {
					if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
						select {
						case eofIo <- true:
							break
						case <-ctx.Done():
							return
						}
						// wait for decision
						select {
						case shallDie := <-die:
							if shallDie {
								return
							}
						case <-ctx.Done():
							return
						}
						continue
					} else {
						select {
						case errorChan <- err:
						case <-ctx.Done():
						}
					}
					return
				}

				select {
				case yieldChan <- &logMessage:
				case <-ctx.Done():
					return
				}
				curOffset++
				if curOffset%everyX == 0 {
					glog.V(2).Infof("Streamed line num %d from %v", curOffset, jobFile)
				}
			}
		}
	}(readerCtx)

	// Main control loop
	for {
		select {
		case err := <-errorChan:
			glog.V(2).Infof("Error while reading jobFile [%v] message: %v;", jobFile, err)
			return err
		case <-readerCtx.Done():
			glog.V(2).Infof("readerCtx.Done for reader obj %v", jobFile)
			return nil
		}
	}
}
