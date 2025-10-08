package mr

import (
	"os"
	"strconv"
)

type TaskType int

const (
	TaskNone TaskType = iota // wait
	TaskMap
	TaskReduce
	TaskExit
)

type TaskState int

const (
	Pending TaskState = iota
	Running
	Done
)

type RequestArgs struct{} // worker asks for a task (no args)

type RequestReply struct {
	Type     TaskType
	MapID    int
	ReduceID int
	NReduce  int
	Files    []string // Map: one input file; Reduce: can be empty (worker derives mr-*-%d)
}

type ReportArgs struct {
	Type     TaskType
	MapID    int
	ReduceID int
	Success  bool
}

type ReportReply struct{}

// ===== Course skeleton keeps these example types; keep them so main builds fine.

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Return the UNIX-domain socket name used by the coordinator.
// DO NOT change this — main/ depends on it.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
