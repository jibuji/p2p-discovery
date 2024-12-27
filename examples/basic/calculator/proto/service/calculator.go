package service

import (
	"context"
	"fmt"

	"github.com/buji/p2p-discovery/examples/basic/calculator/proto"
	"github.com/jibuji/go-stream-rpc/rpc"
)

const CalculatorProtocolID = "/calculator/1.0.0"

type CalculatorService struct {
	proto.UnimplementedCalculatorServer
	client *proto.CalculatorClient
}

func NewCalculatorService(rpc *rpc.RpcPeer) *CalculatorService {
	svc := &CalculatorService{}
	svc.client = proto.NewCalculatorClient(rpc)
	return svc
}

func (s *CalculatorService) Add(ctx context.Context, req *proto.AddRequest) *proto.AddResponse {
	go func() {
		resp := s.client.Multiply(&proto.MultiplyRequest{A: req.A, B: req.B})
		fmt.Println("client in service Add call Multiply", resp)
	}()
	result := req.A + req.B
	return &proto.AddResponse{Result: result}
}

func (s *CalculatorService) Multiply(ctx context.Context, req *proto.MultiplyRequest) *proto.MultiplyResponse {
	// go func() {
	// 	resp := s.client.Add(&proto.AddRequest{A: req.A, B: -req.B})
	// 	fmt.Println("client in service Multiply call Add", resp)
	// }()
	result := req.A * req.B
	return &proto.MultiplyResponse{Result: result}
}
