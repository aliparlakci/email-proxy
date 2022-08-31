package main

import (
	"context"
	"log"

	pb "github.com/aliparlakci/mailproxy/postaci/protobuf"
)

type mailingServerServer struct {
	pb.UnimplementedMailingServerServer
}

func (m *mailingServerServer) ForwardMail(ctx context.Context, request *pb.ForwardMailRequest) (*pb.ForwardMailResponse, error) {

	log.Printf("%v -> %s\n", request.MailId, request.Recipient)

	return &pb.ForwardMailResponse{
		Error:      "",
		Successful: true,
	}, nil
}
