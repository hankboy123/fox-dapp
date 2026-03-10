package client

import (
	"context"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
)

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

// MultiEventListener 封装多个事件的监听，支持自动重连和可配置策略
type MultiEventListener struct {
	filterer    *ClientFilterer
	handlers    map[reflect.Type]EventHandler
	RetryPolicy RetryPolicy // 公开字段，允许用户在 Start 前修改
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewMultiEventListener 创建监听器实例，使用默认重连策略
func NewMultiEventListener(contractAddr common.Address, backend bind.ContractFilterer) (*MultiEventListener, error) {
	filterer, err := NewClientFilterer(contractAddr, backend)
	if err != nil {
		return nil, err
	}
	return &MultiEventListener{
		filterer:    filterer,
		handlers:    make(map[reflect.Type]EventHandler),
		RetryPolicy: DefaultRetryPolicy(),
	}, nil
}

// RegisterHandler 注册特定事件类型的处理函数
func (m *MultiEventListener) RegisterHandler(typ reflect.Type, handler EventHandler) {
	m.handlers[typ] = handler
}

// Start 开始监听所有已注册的事件（自动重连）
func (m *MultiEventListener) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	// 捕获当前策略快照，避免 Start 后修改策略影响已运行的 goroutine
	policy := m.RetryPolicy

	// AuctionCreated
	if handler, ok := m.handlers[reflect.TypeOf(&ClientAuctionCreated{})]; ok {
		watchFunc := func(opts *bind.WatchOpts) (event.Subscription, interface{}, error) {
			sink := make(chan *ClientAuctionCreated)
			sub, err := m.filterer.WatchAuctionCreated(opts, sink, nil, nil, nil)
			return sub, sink, err
		}
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.manageEvent(ctx, watchFunc, handler, policy)
		}()
	}

	// AuctionEnded
	if handler, ok := m.handlers[reflect.TypeOf(&ClientAuctionEnded{})]; ok {
		watchFunc := func(opts *bind.WatchOpts) (event.Subscription, interface{}, error) {
			sink := make(chan *ClientAuctionEnded)
			sub, err := m.filterer.WatchAuctionEnded(opts, sink, nil)
			return sub, sink, err
		}
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.manageEvent(ctx, watchFunc, handler, policy)
		}()
	}

	// BidPlaced
	if handler, ok := m.handlers[reflect.TypeOf(&ClientBidPlaced{})]; ok {
		watchFunc := func(opts *bind.WatchOpts) (event.Subscription, interface{}, error) {
			sink := make(chan *ClientBidPlaced)
			sub, err := m.filterer.WatchBidPlaced(opts, sink, nil, nil)
			return sub, sink, err
		}
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.manageEvent(ctx, watchFunc, handler, policy)
		}()
	}

	// 可按需继续添加其他事件...

	return nil
}

// manageEvent 管理单个事件类型的生命周期，包含可配置的自动重连逻辑
func (m *MultiEventListener) manageEvent(
	ctx context.Context,
	watchFunc func(opts *bind.WatchOpts) (event.Subscription, interface{}, error),
	handler EventHandler,
	policy RetryPolicy,
) {
	retryCount := 0

	for {
		// 每次循环开始检查上下文是否已取消
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 尝试建立订阅
		sub, sink, err := watchFunc(&bind.WatchOpts{Context: ctx})
		if err != nil {
			retryCount++
			if policy.MaxRetries > 0 && retryCount > policy.MaxRetries {
				log.Printf("达到最大重试次数 (%d)，停止监听", policy.MaxRetries)
				return
			}

			// 计算退避时间（指数退避）
			backoff := policy.InitialBackoff
			for i := 1; i < retryCount; i++ {
				backoff = time.Duration(float64(backoff) * policy.Multiplier)
				if backoff > policy.MaxBackoff {
					backoff = policy.MaxBackoff
					break
				}
			}

			log.Printf("订阅失败: %v, 第 %d 次重试，等待 %v", err, retryCount, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		// 订阅成功，重置重试计数
		retryCount = 0

		// 运行事件循环（阻塞直到发生错误或上下文取消）
		m.eventLoop(ctx, sink, sub, handler)

		// 事件循环退出后，检查是否是上下文取消导致的
		if ctx.Err() != nil {
			// 上下文取消，直接退出
			return
		}

		// 非取消退出，说明订阅出错，主动取消订阅
		sub.Unsubscribe()

		// 准备重连，但先等待一个退避时间（这里可以使用固定的短退避，也可以复用之前的退避计算逻辑）
		// 为了简单，这里使用与首次失败相同的退避时间
		backoff := policy.InitialBackoff
		log.Printf("监听断开，等待 %v 后重连", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		// 注意：这里没有增加 retryCount，因为我们把连接断开视为一次新的尝试；如果想要累计错误次数，可以调整逻辑
	}
}

// eventLoop 通用事件循环，使用反射处理不同类型的事件
func (m *MultiEventListener) eventLoop(ctx context.Context, sink interface{}, sub event.Subscription, handler EventHandler) {
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
			log.Println("事件循环上下文取消")
			return
		case 1: // err channel
			if !ok {
				log.Println("错误通道已关闭")
				return
			}
			err, _ := recv.Interface().(error)
			log.Printf("事件循环订阅错误: %v", err)
			return
		case 2: // event channel
			if !ok {
				log.Println("事件通道已关闭")
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
