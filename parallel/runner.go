package parallel

import (
	_ "time"
	"sync"
)

type Runner struct {
	parallel chan struct{}
	tasks    []*task
}

type task struct {
	f   func() error
	err error
}

func NewRunner(maxParallel int) *Runner {
	var parallel chan struct{}
	if maxParallel > 0 {
		parallel = make(chan struct{}, maxParallel)
	}
	tasks := make([]*task, 0, 50)
	runner := &Runner{parallel: parallel, tasks: tasks}
	return runner
}

func (r *Runner) AddTask(f func() error) {
	task := &task{f:f}
	r.tasks = append(r.tasks, task)
}

func (r *Runner) Run() {
	limitedParallel := cap(r.parallel) > 0
	var wg sync.WaitGroup
	for i, t := range r.tasks {
		wg.Add(1)
		if limitedParallel {
			r.parallel <- struct{}{}
		}
		go func(i int, t *task) {
			defer func() {
				wg.Done()
			}()

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

//Returns an array of errors according to the number of tasks or nil if there were no errors
func (r *Runner) Errors() []error {
	var errs []error
	for i, task := range r.tasks {
		if errs == nil {
			errs = make([]error, len(r.tasks))
		}
		errs[i] = task.err
	}
	return errs
}