package main

import (
	"context"
	"fmt"
	pb "github.com/aliparlakci/mailproxy/postaci/protobuf"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gorm.io/gorm"
	"log"
	"net"
	"net/mail"
	"os"
	"path"
	"path/filepath"
	"time"
)

type Email struct {
	gorm.Model
	From     string
	To       string
	Filename string
	SentDate time.Time
	Content  []byte
}

type mailingServerServer struct {
	pb.UnimplementedMailingServerServer
	MailSender
	EmailFinder
}

func (m *mailingServerServer) ForwardMail(ctx context.Context, request *pb.ForwardMailRequest) (*pb.ForwardMailResponse, error) {
	start := time.Now()

	mailId := request.MailId
	recipient := request.Recipient

	email, err := m.EmailFinder.FindEmail(mailId)
	if err != nil {
		logrus.Errorf("something happened while fetching mail content from database: %s", err)
		return &pb.ForwardMailResponse{
			Error:      err.Error(),
			Successful: false,
		}, err
	}

	sender := email.From
	content := email.Content

	err = m.MailSender.Send(ctx, sender, recipient, content)
	if err != nil {
		logrus.Errorf("something happened while sending mail: %s", err)
		return &pb.ForwardMailResponse{
			Error:      err.Error(),
			Successful: false,
		}, err
	}

	elapsed := time.Since(start)
	logrus.WithFields(logrus.Fields{
		"mailId":    mailId,
		"recipient": recipient,
		"sender":    sender,
		"elapsed":   elapsed,
	}).Info("mail forwarded")
	return &pb.ForwardMailResponse{
		Error:      "",
		Successful: true,
	}, nil
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

func ListenIncomingEmails(postfixPath string, cb func(string)) {
	newEmailsPath := path.Join(postfixPath, "new")

	filepath.Walk(newEmailsPath, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}

		if info.IsDir() {
			return err
		}

		if err != nil {
			logrus.WithField("filename", path).Errorf("something happened while reading the directory: %s", err)
		}
		go cb(path)
		return err
	})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("NewWatcher failed: ", err)
	}
	defer watcher.Close()

	err = watcher.Add(newEmailsPath)
	if err != nil {
		log.Fatal("Add failed:", err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			logrus.WithField("filename", path.Base(event.Name)).WithField("op", event.Op.String()).Debug("change in the mail directory")
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

func OnNewEmail(producer MessageProducer) func(string) {
	return func(filepath string) {
		start := time.Now()

		var err error

		var email Email
		if email, err = ReadAndParseEmailFile(filepath); err != nil {
			return
		}

		var emailId uint
		if emailId, err = PersistEmail(email); err != nil {
			log.Printf("Cannot persist the email to DB: %s\n", err)
			return
		}

		if err = EmitNewEmailMessage(producer, emailId, email.To); err != nil {
			log.Printf("Cannot produce new email message: %s\n", err)
			return
		}

		if err = MarkEmailAsRead(filepath); err != nil {
			log.Printf("Cannot mark the email as read: %s\n", err)
			return
		}

		elapsed := time.Since(start)
		logrus.WithFields(logrus.Fields{
			"filename": email.Filename,
			"emailId":  emailId,
			"to":       email.To,
			"elapsed":  elapsed,
		}).Infof("mail is processed as received mail")
	}
}

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	postfixPath := os.Getenv("POSTFIX_PATH")
	kafkaAddress := os.Getenv("KAFKA_ADDRESS")
	kafkaUsername := os.Getenv("KAFKA_USERNAME")
	kafkaPassword := os.Getenv("KAFKA_PASSWORD")

	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetReportCaller(false)
	logrus.SetFormatter(&logrus.TextFormatter{PadLevelText: true})

	persistence := &Persistence{}
	persistence.Initialize(dsn)

	messageBroker := &MessageBroker{}
	messageBroker.Initialize(kafkaAddress, kafkaUsername, kafkaPassword)

	mailSender := &SMPTService{
		Host: "127.0.0.1",
		Port: 25,
	}

	go ListenIncomingEmails(postfixPath, OnNewEmail(messageBroker))

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", 5000))
	if err != nil {
		logrus.Fatal("Failed to create a listener on port 5000")
	}
	server := grpc.NewServer()
	pb.RegisterMailingServerServer(server, &mailingServerServer{MailSender: mailSender, EmailFinder: persistence})
	if err = server.Serve(listener); err != nil {
		logrus.Fatal("Failed to listen")
	}
}
