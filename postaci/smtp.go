package main

import (
	"context"
	"fmt"
	"net/smtp"
)

type SMPTService struct {
	Host     string
	Port     uint16
	Username string
	Password string
}

type MailSender interface {
	Send(ctx context.Context, senderAddr, recipientAddr string, mail []byte) error
}

func (s *SMPTService) Send(ctx context.Context, senderAddr, recipientAddr string, mail []byte) error {
	addr := s.getAddr()

	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("something happened while connecting to the smtp server: %s", err)
	}
	defer c.Close()

	err = c.Mail(senderAddr)
	if err != nil {
		return fmt.Errorf("something happened while issuing a MAIL command: %s", err)
	}

	err = c.Rcpt(recipientAddr)
	if err != nil {
		return fmt.Errorf("something happened while issuing a RCPT command: %s", err)
	}

	writer, err := c.Data()
	if err != nil {
		return fmt.Errorf("something happened while issuing DATA command: %s", err)
	}

	_, err = writer.Write(mail)
	if err != nil {
		return fmt.Errorf("something happened while writing the mail body: %s", err)
	}

	err = c.Close()
	if err != nil {
		return fmt.Errorf("something happened while closing the connection: %s", err)
	}

	return nil
}

func (s *SMPTService) getAuth() smtp.Auth {
	return smtp.PlainAuth("", s.Username, s.Password, s.Host)
}

func (s *SMPTService) getAddr() string {
	return fmt.Sprintf("%s:%v", s.Host, s.Port)
}
