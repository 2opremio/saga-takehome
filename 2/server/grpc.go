package server

import (
	"context"

	"github.com/2opremio/sagatakehome/2/proto"
)

func NewGRPCServer() proto.CounterServer {
	return &GRPCServer{
		counter: &Counter{},
	}
}

type GRPCServer struct {
	counter *Counter
	proto.UnimplementedCounterServer
}

func (g *GRPCServer) Bump(_ context.Context, request *proto.BumpRequest) (*proto.BumpReply, error) {
	result, err := g.counter.Bump(request.By)
	if err != nil {
		return nil, err
	}
	return &proto.BumpReply{Counter: result}, nil
}
