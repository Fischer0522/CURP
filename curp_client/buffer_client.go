package main

import (
	"bytes"
	"fmt"
	"github/Fischer0522/xraft/curp"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ReqReply struct {
	Val    string
	Seqnum int
	Time   time.Time
}

type BufferClient struct {
	Client *curp.Grpc_client

	Reply        chan *ReqReply
	GetClientKey func() int64

	seq         bool
	psize       int
	reqNum      int
	writes      int
	window      int32
	conflict    int
	syncFreq    int
	conflictKey int64

	reqTime    []time.Time
	launchTime time.Time

	rand *rand.Rand
}

func NewBufferClient(c *curp.Grpc_client, reqNum, psize, conflict, writes int, conflictKey int64, clientId int) *BufferClient {
	bc := &BufferClient{
		Client: c,

		Reply: make(chan *ReqReply, reqNum+1),

		seq:          true,
		psize:        psize,
		reqNum:       reqNum,
		writes:       writes,
		conflict:     conflict,
		conflictKey:  conflictKey,
		GetClientKey: nil,
		reqTime:      make([]time.Time, reqNum+1),
	}
	source := rand.NewSource(time.Now().UnixNano() + int64(clientId))
	bc.rand = rand.New(source)
	return bc
}

func (c *BufferClient) Pipeline(syncFreq int, window int32) {
	c.seq = false
	c.syncFreq = syncFreq
	c.window = window
}

func (c *BufferClient) RegisterReply(val string, seqnum int32) {
	t := time.Now()
	c.Reply <- &ReqReply{
		Val:    val,
		Seqnum: int(seqnum),
		Time:   t,
	}
}

// func (c *BufferClient) Write(key int64, val []byte) {
// 	c.Set(strconv.Itoa(int(key)), string(val))
// 	<-c.Reply
// 	return
// }

// func (c *BufferClient) Read(key int64) []byte {

// 	r := <-c.Reply
// 	return r.Val
// }

// func (c *BufferClient) Scan(key, count int64) []byte {
// 	c.SendScan(key, count)
// 	r := <-c.Reply
// 	return r.Val
// }

// Assumed to be connected
func (c *BufferClient) Loop(wg *sync.WaitGroup) {
	val := bytes.Repeat([]byte{1}, 1024)

	// var cmdM sync.Mutex
	// cmdNum := int32(0)
	// wait := make(chan struct{}, 0)
	// go func() {
	// 	for i := 0; i <= c.reqNum; i++ {
	// 		r := <-c.Reply
	// 		// Ignore first request
	// 		if i != 0 {
	// 			d := r.Time.Sub(c.reqTime[r.Seqnum])
	// 			m := float64(d.Nanoseconds()) / float64(time.Millisecond)
	// 			fmt.Println("Returning:", r.Val)
	// 			fmt.Printf("latency %v\n", m)
	// 		}
	// 		if c.window > 0 {
	// 			cmdM.Lock()
	// 			if cmdNum == c.window {
	// 				cmdNum--
	// 				cmdM.Unlock()
	// 				wait <- struct{}{}
	// 			} else {
	// 				cmdNum--
	// 				cmdM.Unlock()
	// 			}
	// 		}
	// 		if c.seq || (c.syncFreq > 0 && i%c.syncFreq == 0) {
	// 			wait <- struct{}{}
	// 		}
	// 	}
	// 	if !c.seq {
	// 		wait <- struct{}{}
	// 	}
	// }()

	max_latency := 0.0
	min_latency := 1000000000000000000000000000000000.0
	avg_latency := 0.0

	for i := 0; i <= c.reqNum; i++ {
		key := c.genGetKey()
		write := c.randomTrue(c.writes)
		c.reqTime[i] = time.Now()

		// Ignore first request
		if i == 1 {
			c.launchTime = c.reqTime[i]
		}
		if write {
			c.Client.Put(strconv.Itoa(int(key)), string(val))
			// TODO: if the return value != i, something's wrong
		} else {
			c.Client.Get(strconv.Itoa(int(key)))
			// TODO: if the return value != i, something's wrong
		}
		if i != 0 {
			now := time.Now()
			d := now.Sub(c.reqTime[i])
			m := float64(d.Nanoseconds()) / float64(time.Millisecond)
			if m > max_latency {
				max_latency = m
			}
			if m < min_latency {
				min_latency = m
			}
			avg_latency += m
			//fmt.Printf("latency %v\n", m)
		}
		// if c.window > 0 {
		// 	cmdM.Lock()
		// 	if cmdNum == c.window-1 {
		// 		cmdNum++
		// 		cmdM.Unlock()
		// 		<-wait
		// 	} else {
		// 		cmdNum++
		// 		cmdM.Unlock()
		// 	}
		// }
		// if c.seq || (c.syncFreq > 0 && i%c.syncFreq == 0) {
		// 	<-wait
		// }
	}
	avg_latency /= float64(c.reqNum)
	wg.Done()

	// if !c.seq {
	// 	<-wait
	// }

	fmt.Printf("Test took %v\n", time.Since(c.launchTime))
	fmt.Printf("max_latency: %v, min_latency: %v, avg_latency: %v\n", max_latency, min_latency, avg_latency)
}

// func (c *BufferClient) WaitReplies(waitFrom int) {
// 	go func() {
// 		for {
// 			r, err := c.GetReplyFrom(waitFrom)
// 			if err != nil {
// 				c.Println(err)
// 				break
// 			}
// 			if r.OK != defs.TRUE {
// 				c.Println("Faulty reply")
// 				break
// 			}
// 			go func(val state.Value, seqnum int32) {
// 				time.Sleep(c.dt.WaitDuration(c.replicas[waitFrom]))
// 				c.RegisterReply(val, seqnum)
// 			}(r.Value, r.CommandId)
// 		}
// 	}()
// }

func (c *BufferClient) genGetKey() int64 {
	key := int64(uuid.New().Time())

	if c.randomTrue(c.conflict) {
		return c.conflictKey
	}
	return key

}

func (c *BufferClient) randomTrue(prob int) bool {
	if prob >= 100 {
		return true
	}
	if prob > 0 {
		return c.rand.Intn(100) <= prob
	}
	return false
}
