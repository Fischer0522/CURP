package witness

import (
	"github/Fischer0522/xraft/curp/command"
	"github/Fischer0522/xraft/curp/curp_proto"
	"sync"
)

type Witness struct {
	mu         sync.Mutex
	commandMap map[command.ProposeId]*curp_proto.CurpClientCommand
}

func NewWitness() Witness {
	return Witness{
		commandMap: make(map[command.ProposeId]*curp_proto.CurpClientCommand),
	}
}

func (w *Witness) InsertIfNotConflict(cmd *curp_proto.CurpClientCommand) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	// command exists before report conflict
	if w.HasConflictWith(cmd) {
		return true
	}

	proposeId := command.ProposeId{
		SeqId:    cmd.SeqId,
		ClientId: cmd.ClientId,
	}

	w.commandMap[proposeId] = cmd

	return false
}

func (w *Witness) Remove(proposeId command.ProposeId) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.commandMap, proposeId)
}

func (w *Witness) HasConflictWith(cmd *curp_proto.CurpClientCommand) bool {
	for _, c := range w.commandMap {
		if curp_proto.Conflict(c, cmd) {
			return true
		}
	}
	return false
}
