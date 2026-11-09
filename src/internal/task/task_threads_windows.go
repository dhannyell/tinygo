//go:build scheduler.threads && windows

package task

// Stop-the-world without signals: the GC suspends each thread and reads its
// registers. The GC thread must not take a lock that a stopped thread can hold.

import "unsafe"

// threadRegs holds the CONTEXT of a stopped thread, aligned in task_threads.c.
// Registers can hold the only pointer to an object.
type threadRegs [156]uintptr

func stopOtherThreads(current *Task) {
	gcState.Store(gcStateStopped)

	// SuspendThread does not wait, so suspend all threads before the first
	// wait. GetThreadContext returns when the thread is stopped.
	for t := activeTasks; t != nil; t = t.state.QueueNext {
		if t != current {
			tinygo_task_suspend(t.state.thread)
		}
	}
	for t := activeTasks; t != nil; t = t.state.QueueNext {
		if t != current {
			sp := tinygo_task_get_context(t.state.thread, unsafe.Pointer(&t.state.regs))
			if sp == 0 {
				runtimeFatal("could not stop thread for GC")
			}
			t.state.stackBottom = sp
		}
	}
}

func resumeOtherThreads(current *Task) {
	gcState.Store(gcStateResumed)
	for t := activeTasks; t != nil; t = t.state.QueueNext {
		if t != current {
			tinygo_task_resume(t.state.thread)
		}
	}
}

func markThreadRegs(t *Task) {
	start := uintptr(unsafe.Pointer(&t.state.regs))
	markRoots(start, start+unsafe.Sizeof(t.state.regs))
}

//export tinygo_task_suspend
func tinygo_task_suspend(threadID)

//export tinygo_task_get_context
func tinygo_task_get_context(thread threadID, buf unsafe.Pointer) uintptr

//export tinygo_task_resume
func tinygo_task_resume(threadID)
