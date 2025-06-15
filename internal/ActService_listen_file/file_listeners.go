package ActService_listen_file

import (
	"errors"
	"fmt"
	"sync"
)

type LogFileListeners struct {
	mu           sync.Locker
	listenerPool map[string]ListenerSet
}

func NewFileListeners() *LogFileListeners {
	return &LogFileListeners{
		listenerPool: make(map[string]ListenerSet),
		mu:           new(sync.Mutex),
	}
}

func (l *LogFileListeners) CancelLogListeners(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	listeners, ok := l.listenerPool[id]
	if !ok {
		return errors.New(fmt.Sprintf("listeners of job %s do not exist, noop", id))
	}
	err := listeners.CancelEach()
	if err != nil {
		return err
	}
	delete(l.listenerPool, id)
	return nil
}

func (l *LogFileListeners) AddListener(id string, listener *InProgressListener) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	listeners, ok := l.listenerPool[id]
	if !ok {
		listeners = NewListenerSet()
		l.listenerPool[id] = listeners
	}
	finalizer, err := listeners.AddListener(listener)
	if err != nil {
		return nil, err
	}
	return finalizer, err
}
