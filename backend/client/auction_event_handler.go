package client

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	// 生成的合约绑定代码
)

// ==================== 节点池接口与实现 ====================

// NodePool 定义节点选择策略
type NodePool interface {
	// GetNode 返回下一个要连接的节点 URL
	// 如果池为空，应返回空字符串和错误
	GetNode() (string, error)
}

// RoundRobinPool 轮询节点池
type RoundRobinPool struct {
	nodes []string
	mu    sync.Mutex
	index int
}

// NewRoundRobinPool 创建轮询节点池
func NewRoundRobinPool(nodes []string) *RoundRobinPool {
	return &RoundRobinPool{
		nodes: nodes,
		index: 0,
	}
}

// GetNode 实现轮询选择
func (p *RoundRobinPool) GetNode() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.nodes) == 0 {
		return "", fmt.Errorf("节点池为空")
	}
	node := p.nodes[p.index]
	p.index = (p.index + 1) % len(p.nodes)
	return node, nil
}

// RandomPool 随机节点池
type RandomPool struct {
	nodes []string
	rng   *rand.Rand
	mu    sync.Mutex
}

// NewRandomPool 创建随机节点池
func NewRandomPool(nodes []string) *RandomPool {
	return &RandomPool{
		nodes: nodes,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetNode 实现随机选择
func (p *RandomPool) GetNode() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.nodes) == 0 {
		return "", fmt.Errorf("节点池为空")
	}
	return p.nodes[p.rng.Intn(len(p.nodes))], nil
}

// ==================== 事件监听核心 ====================

// EventHandler 是所有事件处理函数的类型别名
type EventHandler func(interface{})

// RetryPolicy 定义自动重连的退避策略
type RetryPolicy struct {
	InitialBackoff time.Duration // 首次重试前的等待时间
	MaxBackoff     time.Duration // 最大等待时间
	Multiplier     float64       // 退避因子（必须 > 1）
	MaxRetries     int           // 最大重试次数，0 表示无限重试
}

// DefaultRetryPolicy 返回默认的重连策略
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		MaxRetries:     0,
	}
}

// MultiEventListener 封装多个事件的监听，支持自动重连和可配置的节点池
type MultiEventListener struct {
	contractAddr common.Address
	nodePool     NodePool // 节点池，用于获取连接URL
	handlers     map[reflect.Type]EventHandler
	RetryPolicy  RetryPolicy // 公开字段，允许用户在 Start 前修改
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewMultiEventListener 创建监听器实例，使用默认重连策略
func NewMultiEventListener(contractAddr common.Address, pool NodePool) *MultiEventListener {
	return &MultiEventListener{
		contractAddr: contractAddr,
		nodePool:     pool,
		handlers:     make(map[reflect.Type]EventHandler),
		RetryPolicy:  DefaultRetryPolicy(),
	}
}

// RegisterHandler 注册特定事件类型的处理函数
func (m *MultiEventListener) RegisterHandler(typ reflect.Type, handler EventHandler) {
	m.handlers[typ] = handler
}

// Start 开始监听所有已注册的事件（自动重连 + 节点池）
func (m *MultiEventListener) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	// 捕获当前策略快照
	policy := m.RetryPolicy

	// AuctionCreated
	if handler, ok := m.handlers[reflect.TypeOf(&ClientAuctionCreated{})]; ok {
		m.wg.Add(1)
		go m.manageEvent(ctx, "AuctionCreated", handler, policy, func(filterer *ClientFilterer, opts *bind.WatchOpts) (event.Subscription, interface{}, error) {
			sink := make(chan *ClientAuctionCreated)
			sub, err := filterer.WatchAuctionCreated(opts, sink, nil, nil, nil)
			return sub, sink, err
		})
	}

	// AuctionEnded
	if handler, ok := m.handlers[reflect.TypeOf(&ClientAuctionEnded{})]; ok {
		m.wg.Add(1)
		go m.manageEvent(ctx, "AuctionEnded", handler, policy, func(filterer *ClientFilterer, opts *bind.WatchOpts) (event.Subscription, interface{}, error) {
			sink := make(chan *ClientAuctionEnded)
			sub, err := filterer.WatchAuctionEnded(opts, sink, nil)
			return sub, sink, err
		})
	}

	// BidPlaced
	if handler, ok := m.handlers[reflect.TypeOf(&ClientBidPlaced{})]; ok {
		m.wg.Add(1)
		go m.manageEvent(ctx, "BidPlaced", handler, policy, func(filterer *ClientFilterer, opts *bind.WatchOpts) (event.Subscription, interface{}, error) {
			sink := make(chan *ClientBidPlaced)
			sub, err := filterer.WatchBidPlaced(opts, sink, nil, nil)
			return sub, sink, err
		})
	}

	// 可按需继续添加其他事件...

	return nil
}

// manageEvent 管理单个事件类型的生命周期，包含自动重连和节点切换
func (m *MultiEventListener) manageEvent(
	ctx context.Context,
	eventName string,
	handler EventHandler,
	policy RetryPolicy,
	watchFunc func(filterer *ClientFilterer, opts *bind.WatchOpts) (event.Subscription, interface{}, error),
) {
	defer m.wg.Done()

	var retryCount int

	for {
		// 检查上下文是否取消
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 从节点池获取一个节点URL
		nodeURL, err := m.nodePool.GetNode()
		if err != nil {
			log.Printf("[%s] 获取节点失败: %v", eventName, err)
			// 如果没有可用节点，根据策略决定是否等待重试
			retryCount++
			if policy.MaxRetries > 0 && retryCount > policy.MaxRetries {
				log.Printf("[%s] 达到最大重试次数 (%d)，停止监听", eventName, policy.MaxRetries)
				return
			}
			backoff := m.calcBackoff(retryCount, policy)
			log.Printf("[%s] 等待 %v 后重试...", eventName, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		// 连接节点并创建 filterer
		filterer, client, err := m.dialAndCreateFilterer(nodeURL)
		if err != nil {
			log.Printf("[%s] 连接节点 %s 失败: %v", eventName, nodeURL, err)
			retryCount++
			if policy.MaxRetries > 0 && retryCount > policy.MaxRetries {
				log.Printf("[%s] 达到最大重试次数 (%d)，停止监听", eventName, policy.MaxRetries)
				return
			}
			backoff := m.calcBackoff(retryCount, policy)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		// 连接成功，重置重试计数
		retryCount = 0
		log.Printf("[%s] 成功连接到节点 %s", eventName, nodeURL)

		// 尝试建立订阅
		sub, sink, err := watchFunc(filterer, &bind.WatchOpts{Context: ctx})
		if err != nil {
			log.Printf("[%s] 订阅失败: %v", eventName, err)
			client.Close()
			retryCount++
			if policy.MaxRetries > 0 && retryCount > policy.MaxRetries {
				log.Printf("[%s] 达到最大重试次数 (%d)，停止监听", eventName, policy.MaxRetries)
				return
			}
			backoff := m.calcBackoff(retryCount, policy)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		// 运行事件循环（阻塞直到发生错误或上下文取消）
		m.eventLoop(ctx, sink, sub, handler, eventName)

		// 事件循环退出后，清理当前连接
		sub.Unsubscribe()
		client.Close()

		// 检查是否是上下文取消导致的
		if ctx.Err() != nil {
			return
		}

		// 非取消退出，准备重连（使用初始退避时间）
		log.Printf("[%s] 监听断开，等待 %v 后重试...", eventName, policy.InitialBackoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(policy.InitialBackoff):
		}
	}
}

// dialAndCreateFilterer 连接指定节点并创建 filterer
func (m *MultiEventListener) dialAndCreateFilterer(nodeURL string) (*ClientFilterer, *ethclient.Client, error) {
	client, err := ethclient.Dial(nodeURL)
	if err != nil {
		return nil, nil, err
	}
	filterer, err := NewClientFilterer(m.contractAddr, client)
	if err != nil {
		client.Close()
		return nil, nil, err
	}
	return filterer, client, nil
}

// calcBackoff 根据重试次数和策略计算退避时间
func (m *MultiEventListener) calcBackoff(retryCount int, policy RetryPolicy) time.Duration {
	backoff := policy.InitialBackoff
	for i := 1; i < retryCount; i++ {
		backoff = time.Duration(float64(backoff) * policy.Multiplier)
		if backoff > policy.MaxBackoff {
			backoff = policy.MaxBackoff
			break
		}
	}
	return backoff
}

// eventLoop 通用事件循环，使用反射处理不同类型的事件
func (m *MultiEventListener) eventLoop(ctx context.Context, sink interface{}, sub event.Subscription, handler EventHandler, eventName string) {
	chVal := reflect.ValueOf(sink)
	errCh := reflect.ValueOf(sub.Err()) // sub.Err() 返回 <-chan error

	for {
		cases := []reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
			{Dir: reflect.SelectRecv, Chan: errCh},
			{Dir: reflect.SelectRecv, Chan: chVal},
		}
		chosen, recv, ok := reflect.Select(cases)

		switch chosen {
		case 0: // ctx.Done()
			log.Printf("[%s] 事件循环上下文取消", eventName)
			return
		case 1: // err channel
			if !ok {
				log.Printf("[%s] 错误通道已关闭", eventName)
				return
			}
			err, _ := recv.Interface().(error)
			log.Printf("[%s] 事件循环订阅错误: %v", eventName, err)
			return
		case 2: // event channel
			if !ok {
				log.Printf("[%s] 事件通道已关闭", eventName)
				return
			}
			handler(recv.Interface())
		}
	}
}

// Stop 停止所有监听并等待 goroutine 结束
func (m *MultiEventListener) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}
