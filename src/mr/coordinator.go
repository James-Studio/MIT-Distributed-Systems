package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type taskInfo struct {
	Type     TaskType
	State    TaskState
	MapID    int
	ReduceID int
	Files    []string
	Start    time.Time
}

type Coordinator struct {
	mu      sync.Mutex
	maps    []taskInfo
	reduces []taskInfo
	nReduce int
	done    bool
}

func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := &Coordinator{nReduce: nReduce}
	for i, f := range files {
		c.maps = append(c.maps, taskInfo{
			Type:  TaskMap,
			State: Pending,
			MapID: i,
			Files: []string{f},
		})
	}
	for r := 0; r < nReduce; r++ {
		c.reduces = append(c.reduces, taskInfo{
			Type:     TaskReduce,
			State:    Pending,
			ReduceID: r,
		})
	}
	c.server()
	return c
}

// RPC: workers ask for a task.
func (c *Coordinator) RequestTask(_ *RequestArgs, rep *RequestReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Reap timed-out running tasks (10s).
	reap := func(ts []taskInfo) {
		for i := range ts {
			if ts[i].State == Running && now.Sub(ts[i].Start) > 10*time.Second {
				ts[i].State = Pending
			}
		}
	}
	reap(c.maps)
	reap(c.reduces)

	// If any Map is pending, assign a Map.
	for i := range c.maps {
		if c.maps[i].State == Pending {
			c.maps[i].State = Running
			c.maps[i].Start = now
			*rep = RequestReply{
				Type:    TaskMap,
				MapID:   c.maps[i].MapID,
				NReduce: c.nReduce,
				Files:   c.maps[i].Files,
			}
			return nil
		}
	}

	// If all Maps are done, assign Reduce if pending.
	if allDone(c.maps) {
		for i := range c.reduces {
			if c.reduces[i].State == Pending {
				c.reduces[i].State = Running
				c.reduces[i].Start = now
				*rep = RequestReply{
					Type:     TaskReduce,
					ReduceID: c.reduces[i].ReduceID,
					NReduce:  c.nReduce,
					// Files optional for Reduce; worker will glob mr-*-%d
				}
				return nil
			}
		}
		// If all Reduces are done → tell worker to exit and mark coordinator done.
		if allDone(c.reduces) {
			c.done = true
			*rep = RequestReply{Type: TaskExit}
			return nil
		}
	}

	// Nothing to do right now; ask worker to wait/sleep.
	*rep = RequestReply{Type: TaskNone}
	return nil
}

// RPC: workers report completion (or failure).
func (c *Coordinator) ReportTask(args *ReportArgs, _ *ReportReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch args.Type {
	case TaskMap:
		for i := range c.maps {
			if c.maps[i].MapID == args.MapID {
				if args.Success {
					c.maps[i].State = Done
				} else {
					// put back to pending to reassign
					c.maps[i].State = Pending
				}
				break
			}
		}
	case TaskReduce:
		for i := range c.reduces {
			if c.reduces[i].ReduceID == args.ReduceID {
				if args.Success {
					c.reduces[i].State = Done
				} else {
					c.reduces[i].State = Pending
				}
				break
			}
		}
	}
	return nil
}

func allDone(ts []taskInfo) bool {
	for i := range ts {
		if ts[i].State != Done {
			return false
		}
	}
	return true
}

// main/mrcoordinator.go polls this; return true to exit.
func (c *Coordinator) Done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.done
}

// Server wires up the RPC server exactly as the course skeleton expects.
func (c *Coordinator) server() {
	_ = rpc.Register(c)
	rpc.HandleHTTP()

	// Remove any previous stale socket.
	sockname := coordinatorSock()
	_ = os.Remove(sockname)

	l, err := net.Listen("unix", sockname)
	if err != nil {
		log.Fatal("listen error:", err)
	}

	// Serve RPC requests in a background goroutine.
	go http.Serve(l, nil)
}
