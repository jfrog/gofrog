package parallel

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestTask(t *testing.T) {
	const count = 70
	results := make(chan int, 100)

	runner := NewRunner(4, count, false)
	var expectedTotal int
	var expectedErrorTotal int
	for i := 0; i < count; i++ {
		expectedTotal += i
		if float64(i) > math.Floor(float64(count)/2) {
			expectedErrorTotal += i
		}

		x := i
		runner.AddTask(func(i int) error {
			results <- x
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(50)))
			if float64(x) > math.Floor(float64(count)/2) {
				return fmt.Errorf("Second half value %d not counted", x)
			}
			return nil
		})
	}
	runner.Done()
	runner.Run()

	errs := runner.Errors()

	close(results)
	var resultsTotal int
	for result := range results {
		resultsTotal += result
	}
	if resultsTotal != expectedTotal {
		t.Error("Unexpected results total:", resultsTotal)
	}

	var errorsTotal int
	for k, v := range errs {
		if v != nil {
			errorsTotal += k
		}
	}
	if errorsTotal != expectedErrorTotal {
		t.Error("Unexpected errs total:", errorsTotal)
	}
	if errorsTotal == 0 {
		t.Error("Unexpected 0 errs total")
	}
}
