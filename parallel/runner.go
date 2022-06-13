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

	cancel             chan struct{}
	cancelOnce         sync.Once
	maxParallel        int
	failFast           bool
	threadsWaitGroup   sync.WaitGroup
	threadCount        uint32
	runningThreads     int
	runningThreadsLock sync.Mutex

	idle       sync.Map
	lastActive sync.Map

	errors     map[int]error
	errorsLock sync.Mutex
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
// If a task returns an error and failFast is on all goroutines will stop and the runner will be notified.
// Notice: Run() is a blocking operation.
func (r *runner) Run() {
	for i := 0; i < r.maxParallel; i++ {
		r.addThread()
	}
	r.threadsWaitGroup.Wait()
}

// The producer notifies that no more tasks will be produced.
func (r *runner) Done() {
	close(r.tasks)
}

// Cancel stops the Runner from getting new tasks and empties the tasks queue.
// No new tasks will be executed, and tasks that already started will continue running and won't be interrupted.
// If this Runner was already cancelled, then this function will do nothing.
func (r *runner) Cancel() {
	r.cancelOnce.Do(func() {
		// No more adding tasks
		close(r.cancel)
		// Consume all tasks left
		for len(r.tasks) > 0 {
			<-r.tasks
		}
	})
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
		allIdle := true
		var e error
		r.idle.Range(func(key, value interface{}) bool {
			threadId, ok := key.(int)
			if !ok {
				e = errors.New("thread ID must be a number")
				// this will break iteration
				return false
			}
			threadIdle, ok := value.(bool)
			if !ok {
				e = errors.New("thread idle value must be a boolean")
				// this will break iteration
				return false
			}

			if !threadIdle {
				allIdle = false
				return false
			}

			lastActiveValue, _ := r.lastActive.Load(threadId)
			threadLastActive, _ := lastActiveValue.(string)
			idleTimestamp, err := strconv.ParseInt(threadLastActive, 10, 64)
			if err != nil {
				e = errors.New("unexpected idle timestamp on consumer. err: " + err.Error())
				return false
			}

			idleTime := time.Unix(idleTimestamp, 0)
			if time.Now().Sub(idleTime).Seconds() < float64(idleThresholdSeconds) {
				allIdle = false
				return false
			}
			return true
		})
		if e != nil {
			return e
		}

		// All consumers are idle for the required time.
		if allIdle {
			close(r.tasks)
			return nil
		}
	}
}

func (r *runner) RunningThreads() int {
	return r.runningThreads
}

func (r *runner) SetMaxParallel(newVal int) {
	if newVal < 1 {
		newVal = 1
	}
	if newVal == r.maxParallel {
		return
	}
	if newVal > r.maxParallel {
		for i := 0; i < newVal-r.maxParallel; i++ {
			r.addThread()
		}
	}
	r.maxParallel = newVal
}

func (r *runner) addThread() {
	r.threadsWaitGroup.Add(1)
	nextThreadId := atomic.AddUint32(&r.threadCount, 1) - 1
	go func(threadId int) {
		defer r.threadsWaitGroup.Done()

		r.runningThreadsLock.Lock()
		r.runningThreads++
		r.runningThreadsLock.Unlock()

		for t := range r.tasks {
			r.idle.Store(threadId, false)
			e := t.run(threadId)
			if e != nil {
				if t.onError != nil {
					t.onError(e)
				}

				r.errorsLock.Lock()
				r.errors[int(t.num)] = e
				r.errorsLock.Unlock()

				if r.failFast {
					r.Cancel()
					break
				}
			}
			r.idle.Store(threadId, true)
			r.lastActive.Store(threadId, strconv.FormatInt(time.Now().Unix(), 10))

			r.runningThreadsLock.Lock()
			if r.runningThreads > r.maxParallel {
				r.runningThreads--
				r.runningThreadsLock.Unlock()
				r.idle.Delete(threadId)
				r.lastActive.Delete(threadId)
				break
			}
			r.runningThreadsLock.Unlock()
		}
	}(int(nextThreadId))
}
