package delivery

import (
	"sort"
	"sync/atomic"
)

type ILoadBalancer interface {
	GetNodeId() uint32
	UpdateHandlers(handlers []IHandler)
}

type IHandler interface {
	calcWorkLoad() int
	id() uint32
}

var (
	NewLoadBalancer = newLoadBalancer
)

type loadBalancer struct {
	handlerIds []uint32
	seek       *uint32
	getId      func() uint32
	n          uint32
	nodeId     uint32
}

func (l loadBalancer) GetNodeId() uint32 {
	return l.getId()
}

func (l *loadBalancer) UpdateHandlers(handlers []IHandler) {
	var (
		n          = len(handlers)
		i, j       int
		id         uint32
		handlerIds []uint32
	)
	if n == 0 {
		id = l.nodeId - 1
		l.getId = func() uint32 {
			//LogErr("###########", nil, l.nodeId, id)
			return id
		}
		return
	} else if n == 1 {
		id = handlers[0].id() - 1
		l.getId = func() uint32 {
			//LogErr("!!!!!!!!!", nil, l.nodeId, id)
			return id
		}
		return
	} else {
		sort.Slice(handlers, func(i, j int) bool {
			return handlers[i].calcWorkLoad() < handlers[j].calcWorkLoad()
		})
		handlerIds = make([]uint32, 0, n*(n/2+1))
		for i = range handlers {
			id = handlers[i].id() - 1
			for j = n - i; j > 0; j-- {
				handlerIds = append(handlerIds, id)
			}
		}
		l.handlerIds = handlerIds
		l.n = uint32(len(l.handlerIds))
		atomic.StoreUint32(l.seek, 0)
		//LogErr("++++++++++++++++++++++++", nil, l.nodeId, l.handlerIds)
		l.getId = func() uint32 {
			//LogErr("-----------------------", nil, l.nodeId, l.handlerIds, *l.seek, l.n)
			return l.handlerIds[atomic.AddUint32(l.seek, 1)%l.n]
		}
	}

}

func newLoadBalancer(nodeId uint32) ILoadBalancer {
	id := nodeId - 1
	return &loadBalancer{
		seek:   new(uint32),
		nodeId: nodeId,
		getId: func() uint32 {
			return id
		},
	}
}
