package chain

import "sync"

type TopicQueue struct {
	chanIn chan interface{}
	// FIXME we need this initial queue to do not loose start messages
	//		because wallet first start sending messages and after starts to listen them
	initSubscription *ConcurrentQueue
	subscriptions    []*ConcurrentQueue

	lock *sync.RWMutex
	quit chan struct{}
	wg   *sync.WaitGroup
}

const defaultQueueBuffSize = 20

func NewTopicQueue() *TopicQueue {
	tq := &TopicQueue{
		chanIn:           make(chan interface{}),
		initSubscription: NewConcurrentQueue(defaultQueueBuffSize),
		subscriptions:    make([]*ConcurrentQueue, 0),

		lock: &sync.RWMutex{},
		quit: make(chan struct{}),
		wg:   &sync.WaitGroup{},
	}
	tq.initSubscription.Start()
	return tq
}

func (tq *TopicQueue) ChanIn() chan<- interface{} {
	return tq.chanIn
}

func (tq *TopicQueue) ChanOut() <-chan interface{} {
	tq.lock.Lock()
	defer tq.lock.Unlock()
	var q *ConcurrentQueue
	if tq.initSubscription != nil {
		q = tq.initSubscription
		tq.initSubscription = nil
	} else {
		q = NewConcurrentQueue(defaultQueueBuffSize)
		q.Start()
	}
	tq.subscriptions = append(tq.subscriptions, q)
	return q.chanOut
}

func (tq *TopicQueue) Start() {
	tq.wg.Add(1)
	go func() {
		defer tq.wg.Done()
		for {
			select {
			case item := <-tq.chanIn:
				tq.lock.RLock()
				if tq.initSubscription != nil {
					tq.initSubscription.ChanIn() <- item
				}
				for _, subscription := range tq.subscriptions {
					subscription.ChanIn() <- item
				}
				tq.lock.RUnlock()
			case <-tq.quit:
				return
			}
		}
	}()
}

func (tq *TopicQueue) Stop() {
	close(tq.quit)
	for _, subscription := range tq.subscriptions {
		subscription.Stop()
	}
	tq.wg.Wait()
}
