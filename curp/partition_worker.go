package curp

import (
	"github/Fischer0522/xraft/curp/command"
	"github/Fischer0522/xraft/curp/curp_proto"
	"github/Fischer0522/xraft/curp/trace"
	"github/Fischer0522/xraft/curp/witness"
	xkv "github/Fischer0522/xraft/kv"
	"sync"

	"go.etcd.io/etcd/raft/v3"
)

type PartitionWorker struct {
	workerId     int
	proposeC     chan<- string
	mu           sync.RWMutex
	kvStore      *xkv.KVStore
	commandBoard *command.CommandBoard
	commandSet   map[command.ProposeId]struct{}
	fastPath     chan *curp_proto.CurpClientCommand
	slowPath     chan *curp_proto.CurpClientCommand
	witness      witness.Witness
	raftState    raft.StateType
}

func NewPartitionWorker(workerId int, proposeC chan<- string, kvStore *xkv.KVStore, raftState raft.StateType) *PartitionWorker {
	commandSet := make(map[command.ProposeId]struct{})
	commandBoard := command.NewBoard()
	fastPath := make(chan *curp_proto.CurpClientCommand, 1000)
	slowPath := make(chan *curp_proto.CurpClientCommand, 1000)

	worker := &PartitionWorker{
		workerId:     workerId,
		proposeC:     proposeC,
		kvStore:      kvStore,
		commandBoard: commandBoard,
		commandSet:   commandSet,
		fastPath:     fastPath,
		slowPath:     slowPath,
		witness:      witness.NewWitness(),
		raftState:    raftState,
	}
	go worker.cmd_worker()
	return worker
}

func (p *PartitionWorker) Lookup(cmd *curp_proto.CurpClientCommand) *curp_proto.CurpReply {
	result := p.Propose(cmd)
	return result
}

func (p *PartitionWorker) Propose(cmd *curp_proto.CurpClientCommand) *curp_proto.CurpReply {
	isConflict := p.witness.InsertIfNotConflict(cmd)
	p.mu.Lock()
	if p.raftState == raft.StateLeader {
		trace.Trace(trace.Leader, p.workerId, "propose command %s,type: %s,conflict: %v", cmd.Key, command.OpFmt[cmd.Op], isConflict)
	} else {
		trace.Trace(trace.Follower, p.workerId, "propose command %s,type: %s,conflict: %v", cmd.Key, command.OpFmt[cmd.Op], isConflict)
	}
	if p.raftState != raft.StateLeader {
		if isConflict {
			// TODO: return and report conflict
			p.mu.Unlock()
			reply := &curp_proto.CurpReply{
				Content:    "",
				StatusCode: curp_proto.CONFLICT,
			}
			return reply
		} else {
			p.mu.Unlock()
			reply := &curp_proto.CurpReply{
				Content:    "",
				StatusCode: curp_proto.ACCEPTED,
			}
			return reply
		}
	}
	buf := cmd.Encode()
	go func() {
		p.fastPath <- cmd
	}()
	if p.raftState == raft.StateLeader {
		// command will update the state machine or get command is conflict
		trace.Trace(trace.Leader, p.workerId, "send propose[clientId:%d seqId: %d] msg to raft node", cmd.ClientId, cmd.SeqId)
		go func() {
			p.proposeC <- buf
		}()

	}
	p.mu.Unlock()
	proposeId := command.ProposeId{
		ClientId: cmd.ClientId,
		SeqId:    cmd.SeqId,
	}
	result := p.commandBoard.WaitForEr(proposeId)
	reply := &curp_proto.CurpReply{
		Content: result,
	}
	if isConflict {
		// TODO return and report conflict
		reply.StatusCode = curp_proto.CONFLICT
	} else {
		reply.StatusCode = curp_proto.ACCEPTED
	}
	return reply
}

func (p *PartitionWorker) WaitSynced(id command.ProposeId) *curp_proto.CurpReply {
	result := p.commandBoard.WaitForAsr(id)
	reply := &curp_proto.CurpReply{
		Content:    result,
		StatusCode: curp_proto.ACCEPTED,
	}
	return reply
}

func (p *PartitionWorker) ChangeRaftState(state raft.StateType) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.raftState = state
}

func (p *PartitionWorker) cmd_worker() {
	for {
		select {
		case cmd := <-p.fastPath:
			trace.Trace(trace.Fast, p.workerId, "got cmd from fast path %v", cmd)
			proposeId := command.ProposeId{
				ClientId: cmd.ClientId,
				SeqId:    cmd.SeqId,
			}
			if _, ok := p.commandSet[proposeId]; ok {
				// command executed before,nothing to do
				trace.Trace(trace.Fast, p.workerId, "cmd already executed %v", cmd)
			} else {
				p.commandSet[proposeId] = struct{}{}
				result := p.executeSync(cmd)
				trace.Trace(trace.Fast, p.workerId, "cmd %v executed,result is %v", cmd, result)
				p.commandBoard.InsertEr(proposeId, result)
			}

		case cmd := <-p.slowPath:
			p.mu.Lock()
			isLeader := p.raftState == raft.StateLeader
			p.mu.Unlock()
			trace.Trace(trace.Slow, p.workerId, "got cmd from slow path %v", cmd)
			if _, ok := p.commandSet[cmd.ProposeId()]; ok && isLeader {
				// command executed before,nothing to do
				trace.Trace(trace.Slow, p.workerId, "cmd already executed %v", cmd)
			} else {
				p.commandSet[cmd.ProposeId()] = struct{}{}
				p.executeAsync(cmd)
			}
			trace.Trace(trace.Slow, p.workerId, "WAIT Notify result in slow path,proposeId:[clientId: %d,seqId: %d]", cmd.ClientId, cmd.SeqId)
			p.commandBoard.NotifyAsr(cmd.ProposeId())
			trace.Trace(trace.Slow, p.workerId, "Notify result in slow path,proposeId:[clientId: %d,seqId: %d]", cmd.ClientId, cmd.SeqId)
			// when command is committed, it can be remove from witness and command set safely
			p.removeRecord(cmd)
		}
	}
}

// TODO: refactor it later
// we don't need to lock this function,because it's the only one which modify KVStore
// same as executeAsync
func (p *PartitionWorker) executeSync(cmd *curp_proto.CurpClientCommand) string {
	if cmd.Op == command.PUT {
		// s.kvStore[cmd.Key] = cmd.Value
		(*p.kvStore).Put(cmd.Key, cmd.Value)
	} else if cmd.Op == command.DELETE {
		// delete(s.kvStore, cmd.Key)
		(*p.kvStore).Delete(cmd.Key)
	} else if cmd.Op == command.GET {
		result, err := (*p.kvStore).Get(cmd.Key)
		// result, ok := s.kvStore[cmd.Key]
		if err != nil {
			return "NOT FOUND IN STATE"

		} else {
			return result
		}
	}
	return ""
}

func (p *PartitionWorker) executeAsync(cmd *curp_proto.CurpClientCommand) string {
	if cmd.Op == command.PUT {
		// s.kvStore[cmd.Key] = cmd.Value
		(*p.kvStore).Put(cmd.Key, cmd.Value)
	} else if cmd.Op == command.DELETE {
		// delete(s.kvStore, cmd.Key)
		(*p.kvStore).Delete(cmd.Key)
	} else if cmd.Op == command.GET {
		// return s.kvStore[cmd.Key]
		result, _ := (*p.kvStore).Get(cmd.Key)
		return result
	}
	return ""
}

func (p *PartitionWorker) removeRecord(cmd *curp_proto.CurpClientCommand) {
	// when the command is executed in slow path it means that it has been replicated to the more than 1/2 followers
	// so we can remove it from witness safely
	p.witness.Remove(cmd.ProposeId())

	// since we only support idempotent command like set x = 5 (set x = x + 5 or append command is non-idempotent),
	// we don't need to worry about that some commands will be executed twice
	// so we can remove it from commandSet safely
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.commandSet, cmd.ProposeId())
}
func (p *PartitionWorker) HandleSlowPath(cmd *curp_proto.CurpClientCommand) {
	p.slowPath <- cmd
}
