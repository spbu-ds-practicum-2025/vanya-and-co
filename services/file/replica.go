package file

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ReplicaCluster struct {
	nodes []string // список путей к папкам-узлов (node1, node2...)
	mu    sync.RWMutex
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
	var wg sync.WaitGroup

	for _, nodePath := range c.nodes {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			// Симуляция задержки сети
			time.Sleep(time.Duration(50+rand.Intn(200)) * time.Millisecond)

			// Полный путь: .../data/node1/username/file.txt
			dst := filepath.Join(p, relPath)

			// ВАЖНО: Создаем папку пользователя внутри ноды, иначе WriteFile упадет
			dir := filepath.Dir(dst)
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				log.Printf("Failed to create directory %s: %v", dir, err)
				return
			}

			// Запись
			c.mu.Lock()
			if err := os.WriteFile(dst, data, os.ModePerm); err != nil {
				log.Printf("Failed to write file %s: %v", dst, err)
			} else {
				log.Printf("File written successfully: %s", dst)
			}
			c.mu.Unlock()
		}(nodePath)
	}
	wg.Wait()
}

// прочитать файл с любого узла
func (c *ReplicaCluster) ReadAny(relPath string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, nodePath := range c.nodes {
		path := filepath.Join(nodePath, relPath)
		if data, err := os.ReadFile(path); err == nil {
			log.Printf("File read successfully: %s", path)
			return data, true
		} else {
			log.Printf("File not found at: %s (%v)", path, err)
		}
	}
	log.Printf("File not found in any node: %s", relPath)
	return nil, false
}

// удалить файл со всех узлов
func (c *ReplicaCluster) Delete(relPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, nodePath := range c.nodes {
		path := filepath.Join(nodePath, relPath)
		_ = os.Remove(path)
	}
}
