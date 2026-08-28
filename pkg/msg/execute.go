package msg

import (
	"sync"
	"time"
)

// Result represents the result of a command execution.
type Result struct {
	Value any
	Error error
}

// Execute will run the provided function for all devices in the list with the
// specified level of parallelism. The function is called with the index of the
// device in the provided list. If provided, the done function is called with
// every result as soon as it is available, potentially from multiple goroutines.
// It will return a list of results in the same order as the provided device
// list.
func Execute(list []Device, parallel int, fn func(i int, s *Session) (any, error), done func(i int, result Result)) []Result {
	// ensure parallelism is at least 1
	if parallel < 1 {
		parallel = 1
	}

	// prepare results
	results := make([]Result, len(list))

	// prepare queue
	queue := make(chan int, len(list))
	for i := range list {
		queue <- i
	}
	close(queue)

	// create work group
	var wg sync.WaitGroup
	var mu sync.Mutex

	// add workers
	wg.Add(parallel)

	// spawn workers
	for j := 0; j < parallel; j++ {
		go func() {
			defer wg.Done()

			for i := range queue {
				// yield
				val, err := execute(list[i], func(s *Session) (any, error) {
					return fn(i, s)
				})

				// prepare result
				result := Result{Value: val, Error: err}

				// store result
				mu.Lock()
				results[i] = result
				mu.Unlock()

				// yield result if requested
				if done != nil {
					done(i, result)
				}
			}
		}()
	}

	// wait for all to finish
	wg.Wait()

	return results
}

func execute(d Device, fn func(s *Session) (any, error)) (any, error) {
	// open channel
	ch, err := d.Open()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	// open session
	session, err := OpenSession(ch, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = session.End(time.Second)
	}()

	// execute function
	return fn(session)
}
