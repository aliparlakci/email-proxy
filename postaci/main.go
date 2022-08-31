package main

import (
	"fmt"
	pb "github.com/aliparlakci/mailproxy/postaci/protobuf"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
)

func main() {
	dsn := os.Getenv("MYSQL_CONNECTION_STRING")
	postfixPath := os.Getenv("POSTFIX_PATH")
	kafkaAddress := os.Getenv("KAFKA_ADDRESS")
	kafkaUsername := os.Getenv("KAFKA_USERNAME")
	kafkaPassword := os.Getenv("KAFKA_PASSWORD")

	persistence := &Persistence{}
	persistence.Initialize(dsn)

	messageBroker := &MessageBroker{}
	messageBroker.Initialize(kafkaAddress, kafkaUsername, kafkaPassword)

	done := make(chan bool)
	go ListenIncomingEmails(postfixPath, done, OnNewEmail(messageBroker))

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", 5000))
	if err != nil {
		log.Fatal("Failed to create a listener on port 5000")
	}

	server := grpc.NewServer()
	pb.RegisterMailingServerServer(server, &mailingServerServer{})
	if err = server.Serve(listener); err != nil {
		log.Fatal("Failed to listen")
	}

	<-done
}
