package file

import (
	_"io"
	"math/rand"
	"os"
	"path/filepath"
	"time"
  "fmt"
)

type ReplicaCluster struct {
	nodes []string // список папок-узлов
}

// создаём кластер
func NewCluster(base string, n int) *ReplicaCluster {
	nodes := make([]string, n)

	for i := range nodes {
		nodes[i] = filepath.Join(base, "node"+fmt.Sprint(i+1))
		os.MkdirAll(nodes[i], os.ModePerm)
	}

	return &ReplicaCluster{nodes: nodes}
}

// replicate: записать файл на все узлы
func (c *ReplicaCluster) Write(filename string, data []byte) {

	for _, nodePath := range c.nodes {

		go func(p string) {

			// Симуляция задержки сети
			time.Sleep(time.Duration(50+rand.Intn(200)) * time.Millisecond)

			dst := filepath.Join(p, filename)

			// Запись
			os.WriteFile(dst, data, os.ModePerm)

		}(nodePath)
	}
}

// прочитать файл с любого узла
func (c *ReplicaCluster) ReadAny(filename string) ([]byte, bool) {

	for _, nodePath := range c.nodes {

		path := filepath.Join(nodePath, filename)

		if data, err := os.ReadFile(path); err == nil {
			return data, true
		}
	}

	return nil, false
}

// удалить файл со всех узлов
func (c *ReplicaCluster) Delete(filename string) {

	for _, nodePath := range c.nodes {
		path := filepath.Join(nodePath, filename)
		os.Remove(path)
	}
}
