//go:build scheduler.threads && !windows

package task

// Stop-the-world with a signal: the handler of each thread stores its stack
// pointer and waits for the GC to resume.

// threadRegs is empty: the signal frame on the thread stack holds the
// registers.
type threadRegs struct{}

// scanWaitGroup is used to wait on until all threads have finished the current state transition.
var scanWaitGroup waitGroup

type waitGroup struct {
	f Futex
}

//go:noheap
func (wg *waitGroup) reset(n uint32) {
	wg.f.Store(n)
}

//go:noheap
func (wg *waitGroup) done() {
	if wg.f.Add(^uint32(0)) == 0 {
		wg.f.WakeAll()
	}
}

//go:noheap
func (wg *waitGroup) wait() {
	for {
		val := wg.f.Load()
		if val == 0 {
			return
		}
		wg.f.Wait(val)
	}
}

//go:noheap
func stopOtherThreads(current *Task) {
	// Wait for threads to finish resuming.
	scanWaitGroup.wait()

	// Change the gc state to stopped.
	// NOTE: This does not need to be atomic.
	gcState.Store(gcStateStopped)

	// Set the number of threads to wait for.
	scanWaitGroup.reset(otherTasks(current))

	// Pause all other threads.
	for t := activeTasks; t != nil; t = t.state.QueueNext {
		if t != current {
			tinygo_task_send_gc_signal(t.state.thread)
		}
	}

	// Wait for the threads to finish stopping.
	scanWaitGroup.wait()
}

//go:noheap
func resumeOtherThreads(current *Task) {
	// Set the wait group to track resume progress.
	scanWaitGroup.reset(otherTasks(current))

	// Set the state to resumed.
	gcState.Store(gcStateResumed)

	// Wake all of the stopped threads.
	gcState.WakeAll()
}

//go:noheap
func markThreadRegs(t *Task) {}

//export tinygo_task_gc_pause
func tingyo_task_gc_pause(sig int32) {
	// Write the entrty stack pointer to the state.
	Current().state.stackBottom = uintptr(stacksave())

	// Notify the GC that we are stopped.
	scanWaitGroup.done()

	// Wait for the GC to resume.
	for gcState.Load() == gcStateStopped {
		gcState.Wait(gcStateStopped)
	}

	// Notify the GC that we have resumed.
	scanWaitGroup.done()
}

// Pause the thread by sending it a signal.
//
//export tinygo_task_send_gc_signal
func tinygo_task_send_gc_signal(threadID)
