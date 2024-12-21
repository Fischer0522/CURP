package kv

import (
	"encoding/json"
	"fmt"

	cmap "github.com/orcaman/concurrent-map/v2"
)

// thread safe memory kv
type MemStore struct {
	kv cmap.ConcurrentMap[string, string]
}

func NewMemStore() (*MemStore, error) {
	mkv := &MemStore{}
	mkv.kv = cmap.New[string]()
	return mkv, nil
}

func (m *MemStore) Get(k string) (string, error) {
	v, ok := m.kv.Get(k)
	if ok {
		return v, nil
	} else {
		return v, fmt.Errorf("key not found")
	}
}

func (m *MemStore) Put(k string, v string) error {
	m.kv.Set(k, v)
	return nil
}

func (m *MemStore) Delete(k string) error {
	m.kv.Remove(k)
	return nil
}

func (m *MemStore) PutWithOldValue(k string, v string) (string, error) {
	old_v, ok := m.kv.Get(k)
	m.kv.Set(k, v)
	if ok {
		return old_v, nil
	} else {
		return "", fmt.Errorf("key not found")
	}
}

func (m *MemStore) RollbackPutWithOldValue(k, old_v string) error {
	if old_v == "" {
		m.kv.Remove(k)
	} else {
		m.kv.Set(k, old_v)
	}
	return nil
}

func (m *MemStore) RollbackDeleteWithOldValue(k, old_v string) error {
	if old_v == "" {
		m.kv.Remove(k)
	} else {
		m.kv.Set(k, old_v)
	}
	return nil
}

func (m *MemStore) DeleteWithOldValue(k string) (string, error) {
	old_v, ok := m.kv.Get(k)
	m.kv.Remove(k)
	if ok {
		return old_v, nil
	} else {
		return "", fmt.Errorf("key not found")
	}
}
func (m *MemStore) Destroy() {
	m.kv = cmap.New[string]()
}

// func (m *Mem_kvStore) RLock(k string) (string, KvOpStatus) {
// 	old_val := m.kv[k]
// 	// Rlock k
// 	return old_val, SUCCEED
// }

// func (m *Mem_kvStore) WLock(k string) (string, KvOpStatus) {
// 	old_val := m.kv[k]
// 	// Wlock k
// 	return old_val, SUCCEED
// }

func (m *MemStore) Printf() {
	fmt.Printf("%v\n", m.kv)
}

func (m *MemStore) Equal(kv *MemStore) bool {
	for key, val1 := range m.kv.Items() {
		if val2, ok := kv.kv.Get(key); !ok {
			return false
		} else {
			if val1 != val2 {
				return false
			}
		}
	}
	return true
}

func (m *MemStore) GetSnapshot() ([]byte, error) {
	return json.Marshal(m.kv)
}

func (m *MemStore) RecoverFromSnapshot(snapshot []byte) error {
	if err := json.Unmarshal(snapshot, &m.kv); err != nil {
		return err
	}
	return nil
}
