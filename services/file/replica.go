package file

import (
  "math/rand"
  "sync"
  "time"
)

type ReplicaCluster struct {
  nodes []*FileService
}

func NewCluster(n int) *ReplicaCluster {
  nodes := make([]*FileService, n)
  for i := range nodes { nodes[i] = New() }
  return &ReplicaCluster{nodes: nodes}
}

func (c *ReplicaCluster) Write(id string, data []byte) {
  for _, node := range c.nodes {
    go func(n *FileService) {
      // simulate network delay and possible node down
      time.Sleep(time.Duration(50+rand.Intn(200)) * time.Millisecond)
      n.mu.Lock()
      n.store[id] = FileEntry{Data: data}
      n.mu.Unlock()
    }(node)
  }
}

func (c *ReplicaCluster) ReadAny(id string) ([]byte, bool) {
  for _, n := range c.nodes {
    n.mu.RLock()
    v, ok := n.store[id]
    n.mu.RUnlock()
    if ok { return v.Data, true }
  }
  return nil, false
}
