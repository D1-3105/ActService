package ActService_listen_file

import (
	"errors"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"sync"
)

type InProgressListener struct {
	endCause      *EndIterCause
	cancelContext context.CancelFunc
}

func NewInProgressListener(endCause *EndIterCause, cancelContext context.CancelFunc) *InProgressListener {
	return &InProgressListener{
		endCause:      endCause,
		cancelContext: cancelContext,
	}
}

func (listener *InProgressListener) ForceCancel() {
	listener.cancelContext()
}

type ListenerSet struct {
	ScheduleDelete bool
	Mu             sync.Locker
	listeners      map[uuid.UUID]*InProgressListener
}

func NewListenerSet() ListenerSet {
	return ListenerSet{
		listeners: make(map[uuid.UUID]*InProgressListener),
		Mu:        new(sync.RWMutex),
	}
}

func (ls *ListenerSet) AddListener(listener *InProgressListener) (func(), error) {
	if ls.ScheduleDelete {
		return nil, errors.New("cannot add listener to set, probably job was already cancelled")
	}
	ls.Mu.Lock()
	defer ls.Mu.Unlock()
	listenerId := uuid.New()
	ls.listeners[listenerId] = listener
	return InProgressListenerFinalizer(ls, listenerId), nil
}

func (ls *ListenerSet) CancelEach() error {
	if ls.ScheduleDelete {
		return errors.New("cannot cancel twice, probably job was already cancelled")
	}
	ls.Mu.Lock()
	defer ls.Mu.Unlock()
	for _, listener := range ls.listeners {
		listener.endCause.EndIter <- true
	}
	return nil
}

func (ls *ListenerSet) SetExitOnEof() {
	ls.Mu.Lock()
	defer ls.Mu.Unlock()

	for _, listener := range ls.listeners {
		listener.endCause.EndOnEOF = true
	}
}

func (ls *ListenerSet) DeleteListener(removeId uuid.UUID) error {
	ls.Mu.Lock()
	defer ls.Mu.Unlock()
	_, ok := ls.listeners[removeId]
	if !ok {
		return errors.New("cannot delete listener from set, probably listener was already deleted")
	}
	delete(ls.listeners, removeId)
	return nil
}

func InProgressListenerFinalizer(listenerSet *ListenerSet, removeId uuid.UUID) func() {
	return func() {
		err := listenerSet.DeleteListener(removeId)
		if err != nil {
			glog.Errorf("Failed to delete listener from set: %v", err)
		}
	}
}
