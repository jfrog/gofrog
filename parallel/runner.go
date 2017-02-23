package parallel

import (
	"sync"
	"errors"
)

type Runner interface {
	AddTask(TaskFunc) error
	AddTaskWithError(TaskFunc, OnErrorFunc) error
	Run()
	Close()
}

type TaskFunc func(int) error

type OnErrorFunc func(error)

type task struct {
	run     TaskFunc
	onError OnErrorFunc
}

type runner struct {
	tasks       chan *task
	cancel      chan struct{}
	maxParallel int
	failFast    bool
}

// Create new runner.
// maxParallel - number of go routines which do the actually consuming, maxParallel always will be a positive number.
// failFast - is set to true the will stop on first error.
func NewProducerConsumer(maxParallel int, failFast bool) *runner {
	consumers := maxParallel
	if consumers < 1 {
		consumers = 1
	}
	return &runner{
		tasks:       make(chan *task),
		cancel:      make(chan struct{}),
		maxParallel: consumers,
		failFast:    failFast,
	}
}

// Add a task to the producer channel, in case of cancellation event (caused by @StopProducer()) will return non nil error.
func (r *runner) AddTask(t TaskFunc) error {
	taskWrapper := &task{run: t, onError: func(err error) {}}
	return r.addTask(taskWrapper)
}

// t - the actual task which will be performed by the consumer.
// errorHandler - execute on the returned error while running t
func (r *runner) AddTaskWithError(t TaskFunc, errorHandler OnErrorFunc) error {
	taskWrapper := &task{run: t, onError: errorHandler}
	return r.addTask(taskWrapper)
}

//We're only able to add tasks without blocking, as long as there is a free goroutine in the Run() loop to handle it
//TODO: Add an option for a new runner with capacity - number of tasks that can be added before blocking (needs to be >= maxParallel)
//On runner cancel need to nil existing tasks channel
func (r *runner) addTask(t *task) error {
	select {
	case r.tasks <- t:
		return nil
	case <-r.cancel:
		return errors.New("Runner stopped!")
	}
}

// The producer notify that no more task will be produced.
func (r *runner) Close() {
	close(r.tasks)
}

// Run r.maxParallel go routines in order to consume all the tasks
// If a task returns an error and failFast is on all goroutines will stop and a the runner will be notified.
// Notice: Run() is a blocking operation.
func (r *runner) Run() {
	var wg sync.WaitGroup
	var once sync.Once
	for i := 0; i < r.maxParallel; i++ {
		wg.Add(1)
		go func(threadId int) {
			defer func() {
				wg.Done()
			}()
			for t := range r.tasks {
				e := t.run(threadId)
				if e != nil {
					t.onError(e)
					if r.failFast {
						once.Do(r.Cancel)
						break
					}
				}
			}
		}(i)
	}
	wg.Wait()
}

func (r *runner) Cancel() {
	close(r.cancel)
}
