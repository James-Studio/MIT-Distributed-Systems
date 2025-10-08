package mr

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// KeyValue is the pair emitted by mapf and consumed by reducef.
type KeyValue struct {
	Key   string
	Value string
}

// ihash(key) % NReduce chooses the reduce partition.
func ihash(key string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	for {
		req := RequestArgs{}
		rep := RequestReply{}
		ok := call("Coordinator.RequestTask", &req, &rep)
		if !ok {
			// Coordinator may not be ready yet — retry instead of exiting.
			time.Sleep(300 * time.Millisecond)
			continue
		}
		switch rep.Type {
		case TaskMap:
			if len(rep.Files) == 0 || rep.NReduce <= 0 {
				_ = call("Coordinator.ReportTask",
					&ReportArgs{Type: TaskMap, MapID: rep.MapID, Success: false},
					&ReportReply{})
				break
			}
			if err := doMap(rep.MapID, rep.Files[0], rep.NReduce, mapf); err != nil {
				_ = call("Coordinator.ReportTask",
					&ReportArgs{Type: TaskMap, MapID: rep.MapID, Success: false},
					&ReportReply{})
				break
			}
			_ = call("Coordinator.ReportTask",
				&ReportArgs{Type: TaskMap, MapID: rep.MapID, Success: true},
				&ReportReply{})

		case TaskReduce:
			// rep.NReduce may not be used here; keep for symmetry.
			if err := doReduce(rep.ReduceID, rep.NReduce, reducef); err != nil {
				_ = call("Coordinator.ReportTask",
					&ReportArgs{Type: TaskReduce, ReduceID: rep.ReduceID, Success: false},
					&ReportReply{})
				break
			}
			_ = call("Coordinator.ReportTask",
				&ReportArgs{Type: TaskReduce, ReduceID: rep.ReduceID, Success: true},
				&ReportReply{})

		case TaskNone:
			time.Sleep(300 * time.Millisecond)

		case TaskExit:
			return
		}
	}
}

// Map step: read one input file, run mapf, partition by ihash%N, write mr-<map>-<reduce>
func doMap(mapID int, filename string, nReduce int, mapf func(string, string) []KeyValue) error {
	if nReduce <= 0 {
		return fmt.Errorf("nReduce must be > 0")
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	kva := mapf(filename, string(content))

	encs := make([]*json.Encoder, nReduce)
	files := make([]*os.File, nReduce)
	for r := 0; r < nReduce; r++ {
		f, err := os.CreateTemp("", fmt.Sprintf("mr-%d-%d-*", mapID, r))
		if err != nil {
			return err
		}
		files[r] = f
		encs[r] = json.NewEncoder(f)
	}

	for _, kv := range kva {
		r := ihash(kv.Key) % nReduce
		if err := encs[r].Encode(&kv); err != nil {
			return err
		}
	}

	for r := 0; r < nReduce; r++ {
		name := fmt.Sprintf("mr-%d-%d", mapID, r)
		if err := files[r].Close(); err != nil {
			return err
		}
		if err := os.Rename(files[r].Name(), name); err != nil {
			return err
		}
	}
	return nil
}

// Reduce step: read mr-*-%d, sort by key, group, reducef, write mr-out-%d
func doReduce(reduceID int, _ int, reducef func(string, []string) string) error {
	matches, err := filepath.Glob(fmt.Sprintf("mr-*-%d", reduceID))
	if err != nil {
		return err
	}

	kvs := make([]KeyValue, 0, 1024)
	for _, f := range matches {
		file, err := os.Open(f)
		if err != nil {
			return err
		}
		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				file.Close()
				return err
			}
			kvs = append(kvs, kv)
		}
		file.Close()
	}

	sort.Slice(kvs, func(i, j int) bool { return kvs[i].Key < kvs[j].Key })

	tmp, err := os.CreateTemp("", fmt.Sprintf("mr-out-%d-*", reduceID))
	if err != nil {
		return err
	}
	w := bufio.NewWriter(tmp)

	i := 0
	for i < len(kvs) {
		j := i + 1
		for j < len(kvs) && kvs[j].Key == kvs[i].Key {
			j++
		}
		vals := make([]string, 0, j-i)
		for k := i; k < j; k++ {
			vals = append(vals, kvs[k].Value)
		}
		out := reducef(kvs[i].Key, vals)
		if _, err := fmt.Fprintf(w, "%v %v\n", kvs[i].Key, out); err != nil {
			tmp.Close()
			return err
		}
		i = j
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return err
	}
	final := fmt.Sprintf("mr-out-%d", reduceID)
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		return err
	}
	return nil
}

// ===== the RPC helper from the skeleton, with safe dialing behavior

// call sends an RPC to the coordinator and waits for the response.
// returns false if something goes wrong (e.g., coordinator exited).
func call(rpcname string, args interface{}, reply interface{}) bool {
    sockname := coordinatorSock()
    c, err := rpc.DialHTTP("unix", sockname)
    if err != nil {
        return false
    }
    defer c.Close()

    if err := c.Call(rpcname, args, reply); err != nil {
        return false
    }
    return true
}

