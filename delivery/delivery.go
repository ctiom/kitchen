package delivery

import (
	"context"
)

type Order struct {
	Response func(any, error)
	Ctx      context.Context
	Input    []byte
	MenuId   uint16
	DishId   uint16
}

type Deliverable struct {
	Output []byte
	Error  error
}

type Status struct {
	Host    string
	avgTime uint64
	Loading int32
}

type status struct {
	Status
	ramTtlMb uint32
	cpuNum   uint16
	latency  uint16
}

type ClusterStatus struct {
	Hosts []*Status
}

type ILogistic interface {
	ListenOrder(func(context.Context, *Order))
	ListenStatus(func(*Status))
	SwitchHost(host string)
}
