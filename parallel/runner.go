package parallel

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Runner interface {
	AddTask(TaskFunc) (int, error)
	AddTaskWithError(TaskFunc, OnErrorFunc) (int, error)
	Run()
	Done()
	Cancel()
	Errors() map[int]error
}

type TaskFunc func(int) error

type OnErrorFunc func(error)

type task struct {
	run     TaskFunc
	onError OnErrorFunc
	num     uint32
}

type runner struct {
	tasks     chan *task
	taskCount uint32

	cancel      chan struct{}
	maxParallel int
	failFast    bool

	idle       []bool
	lastActive []string

	errors map[int]error
}

// Create a new capacity runner - a runner we can add tasks to without blocking as long as the capacity is not reached.
// maxParallel - number of go routines for task processing, maxParallel always will be a positive number.
// acceptBeforeBlocking - number of tasks that can be added until a free processing goruntine is needed.
// failFast - is set to true the will stop on first error.
func NewRunner(maxParallel int, capacity uint, failFast bool) *runner {
	consumers := maxParallel
	if consumers < 1 {
		consumers = 1
	}
	if capacity < 1 {
		capacity = 1
	}
	r := &runner{
		tasks:       make(chan *task, capacity),
		cancel:      make(chan struct{}),
		maxParallel: consumers,
		failFast:    failFast,
		idle:        make([]bool, consumers),
		lastActive:  make([]string, consumers),
	}
	r.errors = make(map[int]error)
	return r
}

// Create a new single capacity runner - a runner we can only add tasks to as long as there is a free goroutine in the
// Run() loop to handle it.
// maxParallel - number of go routines for task processing, maxParallel always will be a positive number.
// failFast - if set to true the runner will stop on first error.
func NewBounedRunner(maxParallel int, failFast bool) *runner {
	return NewRunner(maxParallel, 1, failFast)
}

// Add a task to the producer channel, in case of cancellation event (caused by @Cancel()) will return non nil error.
func (r *runner) AddTask(t TaskFunc) (int, error) {
	return r.addTask(t, nil)
}

// t - the actual task which will be performed by the consumer.
// onError - execute on the returned error while running t
// Return the task number assigned to t. Useful to collect errors from the errors map (see @Errors())
func (r *runner) AddTaskWithError(t TaskFunc, errorHandler OnErrorFunc) (int, error) {
	return r.addTask(t, errorHandler)
}

func (r *runner) addTask(t TaskFunc, errorHandler OnErrorFunc) (int, error) {
	nextCount := atomic.AddUint32(&r.taskCount, 1)
	task := &task{run: t, num: nextCount - 1, onError: errorHandler}

	select {
	case <-r.cancel:
		return -1, errors.New("Runner stopped!")
	default:
		r.tasks <- task
		return int(task.num), nil
	}
}

// Run r.maxParallel go routines in order to consume all the tasks
// If a task returns an error and failFast is on all goroutines will stop and a the runner will be notified.
// Notice: Run() is a blocking operation.
func (r *runner) Run() {
	var wg sync.WaitGroup
	var m sync.Mutex
	var once sync.Once
	for i := 0; i < r.maxParallel; i++ {
		wg.Add(1)
		go func(threadId int) {
			defer wg.Done()
			for t := range r.tasks {
				r.idle[threadId] = false
				e := t.run(threadId)
				if e != nil {
					if t.onError != nil {
						t.onError(e)
					}
					m.Lock()
					r.errors[int(t.num)] = e
					m.Unlock()
					if r.failFast {
						once.Do(r.Cancel)
						break
					}
				}
				r.idle[threadId] = true
				r.lastActive[threadId] = strconv.FormatInt(time.Now().Unix(), 10)
			}
		}(i)
	}
	wg.Wait()
}

// The producer notifies that no more tasks will be produced.
func (r *runner) Done() {
	close(r.tasks)
}

func (r *runner) Cancel() {
	// No more adding tasks
	close(r.cancel)
	// Consume all tasks left
	for len(r.tasks) > 0 {
		<-r.tasks
	}
}

// Returns a map of errors keyed by the task number
func (r *runner) Errors() map[int]error {
	return r.errors
}

// Define the work as done when all consumers are idle for idleThresholdSeconds.
// The function will wait until all consumers are idle.
// Can be run by the producer as a go routine right after starting to produce.
// CAUTION - Might panic if no task is added on the initial idleThresholdSeconds.
func (r *runner) DoneWhenAllIdle(idleThresholdSeconds int) error {
	for {
		time.Sleep(time.Duration(idleThresholdSeconds) * time.Second)
		for i := 0; i < r.maxParallel; i++ {
			if !r.idle[i] {
				break
			}

			idleTimestamp, err := strconv.ParseInt(r.lastActive[i], 10, 64)
			if err != nil {
				return errors.New("unexpected idle timestamp on consumer. err: " + err.Error())
			}

			idleTime := time.Unix(idleTimestamp, 0)
			if time.Now().Sub(idleTime).Seconds() < float64(idleThresholdSeconds) {
				break
			}

			// All consumers are idle for the required time.
			if i == r.maxParallel-1 {
				close(r.tasks)
				return nil
			}
		}
	}
}
