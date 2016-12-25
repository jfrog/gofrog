package updown

import (
        "testing"
        "fmt"
        "math/rand"
        "time"
)

var src = rand.NewSource(time.Now().UnixNano())
var rnd = rand.New(src)

func TestTask(t *testing.T) {
        const count = 20
        results := make(chan int, 100)

        runner := NewRunner(4)
        var expectedTotal int
        for i := 0; i < count; i++ {
                expectedTotal += i
                x := i
                runner.AddTask(func() {
                        time.Sleep(time.Millisecond * time.Duration(rnd.Intn(50)))
                        fmt.Printf("%d\n", x)
                        results <- x
                })
        }
        runner.Run()

        close(results)
        var resultsTotal int
        for result := range results {
                resultsTotal += result
        }
        if resultsTotal != expectedTotal {
                t.Fail()
        }
}