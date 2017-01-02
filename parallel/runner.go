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
	f func() error
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

func (r *Runner) Run() []error {
	limitedParallel := cap(r.parallel) > 0
	errs := make([]error, len(r.tasks))
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
				errs[i] = err
			}
			if limitedParallel {
				<-r.parallel
			}
		}(i, t)

	}
	wg.Wait()
	return errs
}
