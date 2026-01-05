package msg

import "sync"

// Result represents the result of a command execution.
type Result struct {
	Value any
	Error error
}

// Execute will run the provided function for all devices in the list with the
// specified level of parallelism. It will return a list of results in the same
// order as the provided device list.
func Execute(list []Device, parallel int, fn func(s *Session) (any, error)) []Result {
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
				val, err := execute(list[i], fn)

				// store result
				mu.Lock()
				results[i] = Result{Value: val, Error: err}
				mu.Unlock()
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
	session, err := OpenSession(ch)
	if err != nil {
		return nil, err
	}

	// execute function
	return fn(session)
}
