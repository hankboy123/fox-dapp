package client

import (
	"context"
	"log"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
)

// EventHandler 是所有事件处理函数的类型别名（可根据需要调整）
type EventHandler func(interface{})

// MultiEventListener 封装多个事件的监听
type MultiEventListener struct {
	filterer      *ClientFilterer
	subscriptions []event.Subscription
	handlers      map[reflect.Type]EventHandler
}

// NewMultiEventListener 创建监听器实例
func NewMultiEventListener(contractAddr common.Address, backend bind.ContractFilterer) (*MultiEventListener, error) {
	filterer, err := NewClientFilterer(contractAddr, backend)
	if err != nil {
		return nil, err
	}
	return &MultiEventListener{
		filterer: filterer,
		handlers: make(map[reflect.Type]EventHandler),
	}, nil
}

// RegisterHandler 注册特定事件类型的处理函数
func (m *MultiEventListener) RegisterHandler(typ reflect.Type, handler EventHandler) {
	m.handlers[typ] = handler
}

// Start 开始监听所有已注册的事件
func (m *MultiEventListener) Start(ctx context.Context) error {
	// 为每个事件类型启动独立的订阅
	// AuctionCreated
	if handler, ok := m.handlers[reflect.TypeOf(&ClientAuctionCreated{})]; ok {
		sink := make(chan *ClientAuctionCreated)
		sub, err := m.filterer.WatchAuctionCreated(&bind.WatchOpts{Context: ctx}, sink, nil, nil, nil)
		if err != nil {
			return err
		}
		m.subscriptions = append(m.subscriptions, sub)
		go m.eventLoop(ctx, sink, sub, handler)
	}

	// AuctionEnded
	if handler, ok := m.handlers[reflect.TypeOf(&ClientAuctionEnded{})]; ok {
		sink := make(chan *ClientAuctionEnded)
		sub, err := m.filterer.WatchAuctionEnded(&bind.WatchOpts{Context: ctx}, sink, nil)
		if err != nil {
			return err
		}
		m.subscriptions = append(m.subscriptions, sub)
		go m.eventLoop(ctx, sink, sub, handler)
	}

	// BidPlaced
	if handler, ok := m.handlers[reflect.TypeOf(&ClientBidPlaced{})]; ok {
		sink := make(chan *ClientBidPlaced)
		sub, err := m.filterer.WatchBidPlaced(&bind.WatchOpts{Context: ctx}, sink, nil, nil)
		if err != nil {
			return err
		}
		m.subscriptions = append(m.subscriptions, sub)
		go m.eventLoop(ctx, sink, sub, handler)
	}

	// 可以继续添加其他事件...

	return nil
}

// eventLoop 通用事件循环
func (m *MultiEventListener) eventLoop(ctx context.Context, sink interface{}, sub event.Subscription, handler EventHandler) {
	// 使用反射处理不同类型的事件通道
	chVal := reflect.ValueOf(sink)
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-sub.Err():
			log.Printf("订阅错误: %v", err)
			return
		default:
			// 从通道接收事件（非阻塞+反射）
			cases := []reflect.SelectCase{
				{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.Err())},
				{Dir: reflect.SelectRecv, Chan: chVal},
			}
			chosen, recv, ok := reflect.Select(cases)
			if chosen == 0 { // err channel
				err, _ := recv.Interface().(error)
				log.Printf("事件循环错误: %v", err)
				return
			}
			if !ok {
				log.Println("事件通道已关闭")
				return
			}
			// 调用注册的处理函数
			handler(recv.Interface())
		}
	}
}

// Stop 停止所有订阅
func (m *MultiEventListener) Stop() {
	for _, sub := range m.subscriptions {
		sub.Unsubscribe()
	}
}
