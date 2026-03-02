package client

import (
	"backend/utils"
	"context"
	"log"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
)

type NodeStatus struct {
	URL    string
	Client *ethclient.Client
	Alive  bool
}

type EthClientPool struct {
	mu sync.RWMutex

	nodes []*NodeStatus

	// 写主节点索引（默认 0）
	primaryIdx int

	// 读操作轮询索引
	readIdx int
}

func NewEthClientPool(ctx context.Context, urls []string) (*EthClientPool, *utils.AppError) {
	if len(urls) == 0 {
		return nil, utils.NewAppError(500, "no rpc urls provided")
	}

	nodes := make([]*NodeStatus, 0, len(urls))

	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}

		client, err := ethclient.DialContext(ctx, u)

		if err != nil {
			nodes = append(nodes, &NodeStatus{
				URL:    u,
				Client: nil,
				Alive:  false,
			})
			continue
		}
		nodes = append(nodes, &NodeStatus{
			URL:    u,
			Client: client,
			Alive:  true,
		})
	}
	if len(nodes) == 0 {
		return nil, utils.NewAppError(500, "no node connected successfully")
	}

	p := &EthClientPool{
		nodes:      nodes,
		primaryIdx: 0,
		readIdx:    0,
	}

	return p, nil
}

func (p *EthClientPool) pickReadNode() *NodeStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.nodes)

	for i := 0; i < n; i++ {
		idx := (p.readIdx + i) % n
		node := p.nodes[idx]
		if node.Alive && node.Client != nil {
			p.readIdx = (idx + 1) % n
			return node
		}
	}
	return nil
}

func (p *EthClientPool) pickPrimaryNode() *NodeStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.nodes)

	// 先看当前 primary 是否可用
	if n > 0 && p.primaryIdx < n {
		node := p.nodes[p.primaryIdx]
		if node.Alive && node.Client != nil {
			return node
		}
	}

	// 否则从头找一个可用的，顺便更新 primaryIdx
	for i := 0; i < n; i++ {
		node := p.nodes[i]
		if node.Alive && node.Client != nil {
			log.Printf("[WARN] switch primary node to %s", node.URL)
			p.primaryIdx = i
			return node
		}
	}
	return nil
}

// markNodeDead 标记节点不可用
func (p *EthClientPool) markNodeDead(url string, cause error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, node := range p.nodes {
		if node.URL == url {
			if node.Alive {
				log.Printf("[ERROR] mark node dead, url=%s, err=%v", url, cause)
			}
			node.Alive = false
			return
		}
	}
}
