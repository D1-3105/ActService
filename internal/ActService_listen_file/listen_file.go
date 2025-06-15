package ActService_listen_file

import (
	"context"
	"github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/sebnyberg/protoio"
	"io"
)

type EndIterCause struct {
	EndIter  chan bool
	EndOnEOF bool
}

func ListenFile(
	ctx context.Context,
	jobFile io.Reader,
	readOffset uint32,
	endIterCause *EndIterCause,
	yieldChan chan *actservice.JobLogMessage,
	finalizer func(),
) error {
	defer finalizer()
	defer close(yieldChan)
	defer close(endIterCause.EndIter)

	reader := protoio.NewReader(jobFile)
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
				die <- end
				return
			case eof := <-eofIo:
				shallDie := eof && endIterCause.EndOnEOF
				select {
				case die <- shallDie:
				case <-ctx.Done():
				}
				if shallDie {
					return
				}
			}
		}
	}(ctx)

	// Reader goroutine
	go func(ctx context.Context) {
		var blackHole actservice.JobLogMessage
		// Skip up to readOffset
		for curOffset < int(readOffset) {
			err := reader.ReadMsg(&blackHole)
			if err == io.EOF {
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
		}

		// Actual reading loop
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var logMessage actservice.JobLogMessage
				err := reader.ReadMsg(&logMessage)
				if err != nil {
					if err == io.EOF {
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
					select {
					case errorChan <- err:
					case <-ctx.Done():
					}
					return
				}

				select {
				case yieldChan <- &logMessage:
				case <-ctx.Done():
					return
				}
				curOffset++
			}
		}
	}(readerCtx)

	// Main control loop
	for {
		select {
		case err := <-errorChan:
			return err
		case shallDie := <-die:
			if shallDie {
				return nil
			}
		case <-readerCtx.Done():
			return nil
		}
	}
}
