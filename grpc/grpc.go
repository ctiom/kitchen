package kitchenGrpc

import (
	"context"
	"errors"
	"github.com/go-preform/kitchen"
	"google.golang.org/grpc"
)

func RegisterGrpcServer(server grpc.ServiceRegistrar, serviceDesc grpc.ServiceDesc, menu kitchen.IMenu) error {
	var (
		dishByNames = make(map[string]kitchen.IDish)
	)
	for _, node := range menu.Nodes() {
		if dish, ok := node.(kitchen.IDish); ok {
			dishByNames[dish.Name()] = dish
		}
	}
	for i := range serviceDesc.Methods {
		if dish, ok := dishByNames[serviceDesc.Methods[i].MethodName]; ok {
			serviceDesc.Methods[i].Handler = buildHandler(dish, serviceDesc.Methods[i].MethodName)
		} else {
			return errors.New("dish not found: " + serviceDesc.Methods[i].MethodName)
		}
	}

	serviceDesc.HandlerType = (*kitchen.IMenu)(nil)

	server.RegisterService(&serviceDesc, menu)

	return nil
}

func buildHandler(dish kitchen.IDish, methodName string) func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		in := dish.Input()
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return dish.CookAny(ctx, in)
		}
		info := &grpc.UnaryServerInfo{
			Server:     srv,
			FullMethod: methodName,
		}
		return interceptor(ctx, in, info, dish.CookAny)
	}
}
