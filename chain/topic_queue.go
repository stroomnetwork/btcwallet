package chain

type TopicQueue struct {
	chanIn        chan interface{}
	subscriptions []*ConcurrentQueue
	quit          chan struct{}
}

func NewTopicQueue() *TopicQueue {
	return &TopicQueue{
		chanIn:        make(chan interface{}),
		subscriptions: make([]*ConcurrentQueue, 0),
		quit:          make(chan struct{}),
	}
}

func (tq *TopicQueue) ChanIn() chan<- interface{} {
	return tq.chanIn
}

func (tq *TopicQueue) SubscribeOnChanOut(bufferSize int) <-chan interface{} {
	q := NewConcurrentQueue(bufferSize)
	defer q.Start()
	tq.subscriptions = append(tq.subscriptions, q)
	return q.chanOut
}

func (tq *TopicQueue) Start() {
	go func() {
		for {
			select {
			case item := <-tq.chanIn:
				for _, subscription := range tq.subscriptions {
					subscription.ChanIn() <- item
				}
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
}
