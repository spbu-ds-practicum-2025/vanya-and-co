package file

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

type ReplicaCluster struct {
	nodes []string // список путей к папкам-узлов (node1, node2...)
}

// Создаём кластер
func NewCluster(base string, n int) *ReplicaCluster {
	nodes := make([]string, n)
	for i := range nodes {
		nodes[i] = filepath.Join(base, fmt.Sprintf("node%d", i+1))
		_ = os.MkdirAll(nodes[i], os.ModePerm)
	}
	return &ReplicaCluster{nodes: nodes}
}

// replicate: записать файл на все узлы
// relPath должен иметь формат "username/filename"
func (c *ReplicaCluster) Write(relPath string, data []byte) {
	for _, nodePath := range c.nodes {
		go func(p string) {
			// Симуляция задержки сети
			time.Sleep(time.Duration(50+rand.Intn(200)) * time.Millisecond)

			// Полный путь: .../data/node1/username/file.txt
			dst := filepath.Join(p, relPath)

			// ВАЖНО: Создаем папку пользователя внутри ноды, иначе WriteFile упадет
			_ = os.MkdirAll(filepath.Dir(dst), os.ModePerm)

			// Запись
			_ = os.WriteFile(dst, data, os.ModePerm)
		}(nodePath)
	}
}

// прочитать файл с любого узла
func (c *ReplicaCluster) ReadAny(relPath string) ([]byte, bool) {
	for _, nodePath := range c.nodes {
		path := filepath.Join(nodePath, relPath)
		if data, err := os.ReadFile(path); err == nil {
			return data, true
		}
	}
	return nil, false
}

// удалить файл со всех узлов
func (c *ReplicaCluster) Delete(relPath string) {
	for _, nodePath := range c.nodes {
		path := filepath.Join(nodePath, relPath)
		_ = os.Remove(path)
	}
}