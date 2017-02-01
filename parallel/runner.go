package parallel

import (
	"sync"
)

// Runner general interface to perform a number of tasks in parallel
type Runner interface {
	AddTask(f func() error)
	Run()
	Errors() []error
}

// simpleRunner concrete type implementing the Runner interface
type simpleRunner struct {
	parallel chan struct{}
	tasks    []*task
}

type task struct {
	f   func() error
	err error
}

// NewRunner generates a new runner from a given maximal number of parallel tasks
func NewRunner(maxParallel int) Runner {
	var parallel chan struct{}
	if maxParallel > 0 {
		parallel = make(chan struct{}, maxParallel)
	}
	tasks := make([]*task, 0, 50)
	runner := &simpleRunner{parallel: parallel, tasks: tasks}
	return runner
}

// AddTask creates task from a function and appends it to the runner
func (r *simpleRunner) AddTask(f func() error) {
	task := &task{f: f}
	r.tasks = append(r.tasks, task)
}

// Run executes the tasks, waits for all completions and fills errors if some occur
func (r *simpleRunner) Run() {
	limitedParallel := cap(r.parallel) > 0
	var wg sync.WaitGroup
	for i, t := range r.tasks {
		wg.Add(1)
		if limitedParallel {
			r.parallel <- struct{}{}
		}
		go func(i int, t *task) {
			defer wg.Done()
			if err := t.f(); err != nil {
				t.err = err
			}
			if limitedParallel {
				<-r.parallel
			}
		}(i, t)
	}
	wg.Wait()
}

// Errors returns an array of errors according to the number of tasks or nil if there were no errors
func (r *simpleRunner) Errors() []error {
	var errs []error
	for i, task := range r.tasks {
		if errs == nil {
			errs = make([]error, len(r.tasks))
		}
		errs[i] = task.err
	}
	return errs
}
