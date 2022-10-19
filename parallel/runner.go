package parallel

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

const waitForTasksTime = 10 * time.Second

type Runner interface {
	AddTask(TaskFunc) (int, error)
	AddTaskWithError(TaskFunc, OnErrorFunc) (int, error)
	Run()
	Done()
	Cancel()
	Errors() map[int]error
	ActiveThreads() uint32
	OpenThreads() int
	IsStarted() bool
	SetMaxParallel(int)
	GetFinishedNotification() chan bool
	SetFinishedNotification(bool)
}

type TaskFunc func(int) error

type OnErrorFunc func(error)

type task struct {
	run     TaskFunc
	onError OnErrorFunc
	num     uint32
}

type runner struct {
	// Tasks waiting to be executed.
	tasks chan *task
	// Tasks counter, used to give each task an identifier (task.num).
	taskId uint32
	// A channel that is closed when the runner is cancelled.
	cancel chan struct{}
	// Used to make sure the cancel channel is closed only once.
	cancelOnce sync.Once
	// The maximum number of threads running in parallel.
	maxParallel int
	// If true, the runner will be cancelled on the first error thrown from a task.
	failFast bool
	// Indicates that the runner received some tasks and started executing them.
	started uint32
	// A WaitGroup that waits for all the threads to close.
	threadsWaitGroup sync.WaitGroup
	// Threads counter, used to give each thread an identifier (threadId).
	threadCount uint32
	// The number of open threads.
	openThreads int
	// A lock on openThreads.
	openThreadsLock sync.Mutex
	// The number of threads currently running tasks.
	activeThreads uint32
	// The number of tasks in the queue.
	totalTasksInQueue uint32
	// Indicate that the runner has finished.
	finishedNotifier chan bool
	// Indicates that the finish channel is closed.
	finishedNotifierChannelClosed bool
	// A lock for the finishedNotifier check.
	finishedNotifierLock sync.Mutex
	// A flag that allows receiving a notification through a channel, when the runner finishes executing all the tasks.
	finishedNotificationEnabled bool
	// A map of errors keyed by threadId.
	errors map[int]error
	// A lock on the errors map.
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
		finishedNotifier: make(chan bool, 1),
		maxParallel:      consumers,
		failFast:         failFast,
		cancel:           make(chan struct{}),
		tasks:            make(chan *task, capacity),
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
	nextCount := atomic.AddUint32(&r.taskId, 1)
	task := &task{run: t, num: nextCount - 1, onError: errorHandler}

	select {
	case <-r.cancel:
		return -1, errors.New("runner stopped")
	default:
		atomic.AddUint32(&r.totalTasksInQueue, 1)
		r.tasks <- task
		return int(task.num), nil
	}
}

// Run r.maxParallel go routines in order to consume all the tasks
// If a task returns an error and failFast is on all goroutines will stop and the runner will be notified.
// Notice: Run() is a blocking operation.
func (r *runner) Run() {
	if r.finishedNotificationEnabled {
		// This go routine awaits for an execution of a task. The runner will finish its run if no tasks were executed for waitForTasksTime.
		go func() {
			time.Sleep(waitForTasksTime)
			if !r.IsStarted() {
				r.notifyFinished()
			}
		}()
	}

	for i := 0; i < r.maxParallel; i++ {
		r.addThread()
	}
	r.threadsWaitGroup.Wait()
}

// Done is used to notify that no more tasks will be produced.
func (r *runner) Done() {
	close(r.tasks)
}

// GetFinishedNotification returns the finishedNotifier channel, which notifies when the runner is done.
// In order to use the finishedNotifier channel, you must set the finishedNotificationEnabled variable.
func (r *runner) GetFinishedNotification() chan bool {
	return r.finishedNotifier
}

// IsStarted is true when a task was executed, false otherwise.
func (r *runner) IsStarted() bool {
	return r.started > 0
}

// Cancel stops the Runner from getting new tasks and empties the tasks queue.
// No new tasks will be executed, and tasks that already started will continue running and won't be interrupted.
// If this Runner is already cancelled, then this function will do nothing.
func (r *runner) Cancel() {
	r.cancelOnce.Do(func() {
		// No more adding tasks
		close(r.cancel)
		// Consume all tasks left
		for len(r.tasks) > 0 {
			<-r.tasks
		}
		if r.finishedNotificationEnabled {
			r.notifyFinished()
		}
	})
}

// Errors Returns a map of errors keyed by the task number
func (r *runner) Errors() map[int]error {
	return r.errors
}

// OpenThreads returns the number of open threads (including idle threads).
func (r *runner) OpenThreads() int {
	return r.openThreads
}

func (r *runner) ActiveThreads() uint32 {
	return r.activeThreads
}

func (r *runner) SetFinishedNotification(toEnable bool) {
	r.finishedNotificationEnabled = toEnable
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
	// In case the number of threads is reduced, we set the new value to maxParallel, and each thread that finishes his
	// task checks if there are more open threads than maxParallel. If so, it kills itself.
	r.maxParallel = newVal
}

func (r *runner) addThread() {
	r.threadsWaitGroup.Add(1)
	nextThreadId := atomic.AddUint32(&r.threadCount, 1) - 1
	go func(threadId int) {
		defer r.threadsWaitGroup.Done()
		r.openThreadsLock.Lock()
		r.openThreads++
		r.openThreadsLock.Unlock()

		// Keep on taking tasks from the queue.
		for t := range r.tasks {
			// Increase the total of active threads.
			atomic.AddUint32(&r.activeThreads, 1)
			atomic.AddUint32(&r.started, 1)
			// Run the task.
			e := t.run(threadId)
			// Decrease the total of active threads.
			atomic.AddUint32(&r.activeThreads, ^uint32(0))
			// Decrease the total of in progress tasks.
			atomic.AddUint32(&r.totalTasksInQueue, ^uint32(0))
			if r.finishedNotificationEnabled {
				r.finishedNotifierLock.Lock()
				// Notify that the runner has finished its job.
				if r.activeThreads == 0 && r.totalTasksInQueue == 0 {
					r.notifyFinished()
				}
				r.finishedNotifierLock.Unlock()
			}

			if e != nil {
				if t.onError != nil {
					t.onError(e)
				}

				// Save the error in the errors map.
				r.errorsLock.Lock()
				r.errors[int(t.num)] = e
				r.errorsLock.Unlock()

				if r.failFast {
					r.Cancel()
					break
				}
			}

			r.openThreadsLock.Lock()
			// If the total of open threads is larger than the maximum (maxParallel), then this thread should be closed.
			if r.openThreads > r.maxParallel {
				r.openThreads--
				r.openThreadsLock.Unlock()
				break
			}
			r.openThreadsLock.Unlock()
		}
	}(int(nextThreadId))
}

func (r *runner) notifyFinished() {
	if !r.finishedNotifierChannelClosed {
		r.finishedNotifier <- true
		r.finishedNotifierChannelClosed = true
		close(r.finishedNotifier)
	}
}
