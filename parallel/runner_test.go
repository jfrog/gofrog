package parallel

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var errTest = errors.New("some error")

func TestIsStarted(t *testing.T) {
	runner := NewBounedRunner(1, false)
	_, err := runner.AddTask(func(i int) error {
		return nil
	})
	assert.NoError(t, err)
	runner.Done()
	runner.Run()
	assert.True(t, runner.IsStarted())
}

func TestAddTask(t *testing.T) {
	const count = 70
	results := make(chan int, 100)

	runner := NewRunner(4, count, false)
	var expectedTotal int
	var expectedErrorTotal int
	for i := 0; i < count; i++ {
		expectedTotal += i
		if float64(i) > float64(count)/2 {
			expectedErrorTotal += i
		}

		x := i
		_, err := runner.AddTask(func(int) error {
			results <- x
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(50)))
			if float64(x) > float64(count)/2 {
				return fmt.Errorf("second half value %d not counted", x)
			}
			return nil
		})
		assert.NoError(t, err)
	}
	runner.Done()
	runner.Run()

	close(results)
	var resultsTotal int
	for result := range results {
		resultsTotal += result
	}
	assert.Equal(t, expectedTotal, resultsTotal)

	var errorsTotal int
	for k, v := range runner.Errors() {
		if v != nil {
			errorsTotal += k
		}
	}
	assert.Equal(t, expectedErrorTotal, errorsTotal)
	assert.NotZero(t, errorsTotal)
}

func TestAddTaskWithError(t *testing.T) {
	// Create new runner
	runner := NewRunner(1, 1, false)

	// Add task with error
	var receivedError = new(error)
	onError := func(err error) { *receivedError = err }
	taskFunc := func(int) error { return errTest }
	_, err := runner.AddTaskWithError(taskFunc, onError)
	assert.NoError(t, err)

	// Wait for task to finish
	runner.Done()
	runner.Run()

	// Assert error captured
	assert.Equal(t, errTest, *receivedError)
	assert.Equal(t, errTest, runner.Errors()[0])
}

func TestCancel(t *testing.T) {
	// Create new runner
	runner := NewBounedRunner(1, false)

	// Cancel to prevent receiving another tasks
	runner.Cancel(false)

	// Add task and expect error
	_, err := runner.AddTask(func(int) error { return nil })
	assert.ErrorContains(t, err, "runner stopped")
}

func TestForceCancel(t *testing.T) {
	// Create new runner
	const capacity = 10
	runner := NewRunner(1, capacity, true)
	// Run tasks
	for i := 0; i < capacity; i++ {
		taskId := i
		_, err := runner.AddTask(func(int) error {
			assert.Less(t, taskId, 9)
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		assert.NoError(t, err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runner.Run()
	}()
	go func() {
		time.Sleep(200 * time.Millisecond)
		runner.Cancel(true)
	}()
	wg.Wait()

	assert.InDelta(t, 5, runner.started, 4)
}

func TestFailFast(t *testing.T) {
	// Create new runner with fail-fast
	runner := NewBounedRunner(1, true)

	// Add task that returns an error
	_, err := runner.AddTask(func(int) error {
		return errTest
	})
	assert.NoError(t, err)

	// Wait for task to finish
	runner.Run()

	// Add another task and expect error
	_, err = runner.AddTask(func(int) error {
		return nil
	})
	assert.ErrorContains(t, err, "runner stopped")
}

func TestNotifyFinished(t *testing.T) {
	// Create new runner
	runner := NewBounedRunner(1, false)
	runner.SetFinishedNotification(true)

	// Cancel to prevent receiving another tasks
	runner.Cancel(false)
	<-runner.GetFinishedNotification()
}

func TestMaxParallel(t *testing.T) {
	// Create new runner with capacity of 10 and max parallelism of 3
	const capacity = 10
	const parallelism = 3
	runner := NewRunner(parallelism, capacity, false)

	// Run tasks in parallel
	for i := 0; i < capacity; i++ {
		_, err := runner.AddTask(func(int) error {
			// Assert in range between 1 and 3
			assert.InDelta(t, 2, runner.ActiveThreads(), 1)
			assert.InDelta(t, 2, runner.OpenThreads(), 1)
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		assert.NoError(t, err)
	}

	// Wait for tasks to finish
	runner.Done()
	runner.Run()
	assert.Equal(t, uint32(capacity), runner.started)
}

func TestResetFinishNotificationIfActive(t *testing.T) {
	// Create 2 runners
	const capacity = 10
	const parallelism = 3
	runnerOne := NewRunner(parallelism, capacity, false)
	runnerOne.SetFinishedNotification(true)
	runnerTwo := NewRunner(parallelism, capacity, false)
	runnerTwo.SetFinishedNotification(true)

	// Add 10 tasks to runner one. Each task provides tasks to runner two.
	for i := 0; i < capacity; i++ {
		_, err := runnerOne.AddTask(func(int) error {
			time.Sleep(time.Millisecond * 100)
			_, err := runnerTwo.AddTask(func(int) error {
				time.Sleep(time.Millisecond)
				return nil
			})
			assert.NoError(t, err)
			return nil
		})
		assert.NoError(t, err)
	}

	// Create a goroutine waiting for the finish notification of the first runner before running "Done".
	go func() {
		<-runnerOne.GetFinishedNotification()
		runnerOne.Done()
	}()

	// Start running the second runner in a different goroutine to make it non-blocking.
	go func() {
		runnerTwo.Run()
	}()

	// Run the first runner. This is a blocking method.
	runnerOne.Run()

	// Reset runner two's finish notification to ensure we receive it only after all tasks assigned to runner two are completed.
	runnerTwo.ResetFinishNotificationIfActive()

	// Receive the finish notification and ensure that we have truly completed the task.
	<-runnerTwo.GetFinishedNotification()
	assert.Zero(t, runnerTwo.ActiveThreads())
	runnerTwo.Done()
}
