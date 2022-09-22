package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"testing"
	"time"
)

type FakeMessageProducer struct {
	topic    string
	message  string
	isCalled bool
}

func (f *FakeMessageProducer) Produce(ctx context.Context, topic string, message string) error {
	f.topic = topic
	f.message = message
	f.isCalled = true

	return nil
}

func (f *FakeMessageProducer) IsCalledWith(topic, message string) bool {
	return f.isCalled && f.topic == topic && f.message == message
}

func TestListenNewEmailFile(t *testing.T) {
	directory, _ := os.MkdirTemp(".", "tmp")
	defer func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Fatalf("cannot remove the temp directory: %s", err)
		}
	}()

	err := os.Mkdir(path.Join(directory, "new"), fs.ModePerm)
	if err != nil {
		t.Fatalf("cannot create new directory: %s", err)
	}
	err = os.Mkdir(path.Join(directory, "cur"), fs.ModePerm)
	if err != nil {
		t.Fatalf("cannot create cur directory: %s", err)
	}

	sampleEmail := "Return-Path: <contact@example.com>\nX-Original-To: ali@example.com\nDelivered-To: aliparlakci@DESKTOP-J5126FH.localdomain\nReceived: by DESKTOP-J5126FH.localdomain (Postfix, from userid 1000)\n\tid 12527F456; Thu,  1 Sep 2022 12:59:37 +0000 (UTC)\nSubject: Test email subject line\nTo: <admin@example.com>\nX-Mailer: mail (GNU Mailutils 3.7)\nMessage-Id: <20220901125937.12527F456@DESKTOP-J5126FH.localdomain>\nDate: Thu,  1 Sep 2022 12:59:37 +0000 (UTC)\nFrom: DESKTOP-J5126FH <contact@example.com>\n\nthis is my mail\n"
	sampleEmailPath := path.Join(directory, "new", "sample_email")

	persistence := &Persistence{}
	persistence.InitializeTesting()

	fakeMessageProducer := &FakeMessageProducer{}

	done := make(chan bool)
	timer := time.NewTimer(10 * time.Second)
	go ListenIncomingEmails(fmt.Sprintf("%v/.", directory), func(filename string) {
		OnNewEmail(fakeMessageProducer)(filename)
		done <- true
	})

	if err := os.WriteFile(sampleEmailPath, []byte(sampleEmail), fs.ModePerm); err != nil {
		t.Fatalf("cannot create sample email file: %s", err)
	}

	for {
		select {
		case <-timer.C:
			t.Errorf("hook is not run within 1 second")
			return
		case <-done:
			email, _ := persistence.FindEmail(1)
			if email.To != "ali@example.com" {
				t.Errorf("expected recipient to be %s, but got %s", "ali@example.com", email.To)
			}

			if email.From != "contact@example.com" {
				t.Errorf("expected recipient to be %s, but got %s", "contact@example.com", email.From)
			}

			if email.Filename != "sample_email" {
				t.Errorf("expected recipient to be %s, but got %s", "sample_email", email.From)
			}

			if !fakeMessageProducer.IsCalledWith("newemail", fmt.Sprintf("%v|%s", email.ID, email.To)) {
				t.Errorf("message broker is not called with correct arguments")
			}

			return
		}
	}
}
