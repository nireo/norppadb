package worker

type Queue chan<- Work
type worker chan Work
type Work func()
type dispatcher chan chan Work

func NewQueue(numWorkers int) Queue {
	q := make(chan Work)
	d := make(dispatcher, numWorkers)
	go d.dispatch(q)
	return Queue(q)
}

func (d dispatcher) dispatch(queue <-chan Work) {
	for i := 0; i < cap(d); i++ {
		w := make(worker)
		go w.work(d)
	}

	go func() {
		for work := range queue {
			go func(work Work) {
				worker := <-d
				worker <- work
			}(work)
		}
		for i := 0; i < cap(d); i++ {
			w := <-d
			close(w)
		}
	}()
}

func (w worker) work(d dispatcher) {
	d <- w
	go w.wait(d)
}

func (w worker) wait(d dispatcher) {
	for work := range w {
		work()
		d <- w
	}
}
