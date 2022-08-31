package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/mail"
	"os"
	"path"
	"time"

	"github.com/fsnotify/fsnotify"
	"gorm.io/gorm"
)

type Email struct {
	gorm.Model
	From     string
	To       string
	Filename string
	SentDate time.Time
	Content  []byte
}

func ReadAndParseEmailFile(filepath string) (Email, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return Email{}, err
	}

	message, err := mail.ReadMessage(file)
	if err != nil {
		return Email{}, err
	}
	file.Close()

	sentDate, err := message.Header.Date()
	if err != nil {
		log.Printf("Cannot read the sent date from email. Current time will be used instead: %s", err)
		sentDate = time.Now()
	}

	var receiverAddress mail.Address
	if addresses, err := message.Header.AddressList("X-Original-To"); err == nil {
		receiverAddress = *addresses[0]
	} else {
		log.Printf("Cannot read the sender address: %s", err)
	}

	var senderAddress mail.Address
	if addresses, err := message.Header.AddressList("From"); err == nil {
		senderAddress = *addresses[0]
	} else {
		log.Printf("Cannot read the sender address: %s", err)
	}

	content, err := os.ReadFile(filepath)
	if err != nil {
		log.Printf("Cannot read email file.")
		return Email{}, err
	}

	return Email{
		Content:  content,
		Filename: path.Base(file.Name()),
		To:       receiverAddress.Address,
		From:     senderAddress.Address,
		SentDate: sentDate,
	}, nil
}

func PersistEmail(email Email) (uint, error) {
	result := db.Create(&email)
	return email.ID, result.Error
}

func MarkEmailAsRead(filepath string) error {
	filename := path.Base(filepath)
	newFolderPath := path.Dir(filepath)
	postFixPath := path.Dir(newFolderPath)

	newPath := path.Join(postFixPath, "cur", filename)

	return os.Rename(filepath, newPath)
}

func EmitNewEmailMessage(producer MessageProducer, mailId uint, receiver string) error {
	return producer.Produce(context.Background(), "newemail", fmt.Sprintf("%v|%s", mailId, receiver))
}

func OnNewEmail(producer MessageProducer) func(string) {
	return func(filepath string) {
		var err error

		var email Email
		if email, err = ReadAndParseEmailFile(filepath); err != nil {
			return
		}

		var mailId uint
		if mailId, err = PersistEmail(email); err != nil {
			log.Printf("Cannot persist the email to DB: %s\n", err)
			return
		}

		if err = EmitNewEmailMessage(producer, mailId, email.To); err != nil {
			log.Printf("Cannot produce new email message: %s\n", err)
			return
		}

		if err = MarkEmailAsRead(filepath); err != nil {
			log.Printf("Cannot mark the email as read: %s\n", err)
			return
		}
	}
}

func ListenIncomingEmails(path string, done chan bool, cb func(string)) {
	defer close(done)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("NewWatcher failed: ", err)
	}
	defer watcher.Close()

	err = watcher.Add(path)
	if err != nil {
		log.Fatal("Add failed:", err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op == fsnotify.Create {
				go cb(event.Name)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

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
	if err = server.Serve(listener); err != nil {
		log.Fatal("Failed to listen")
	}

	<-done
}
