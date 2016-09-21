package mq

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/fasthall/cappa/docker"
	"github.com/fasthall/cappa/redis"
	"github.com/nu7hatch/gouuid"
	"github.com/streadway/amqp"
	"os"
)

type MQ struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

type Data struct {
	Id      string
	Payload []byte
}

func (d *Data) Encode() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(d)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Decode(d []byte) (Data, error) {
	buf := bytes.NewBuffer(d)
	dec := gob.NewDecoder(buf)
	var to Data
	err := dec.Decode(&to)
	return to, err
}

func NewMQ() (*MQ, error) {
	mq := new(MQ)
	fmt.Println("Connecting to RabbitMQ...")
	var err error
	mq.conn, err = amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		return nil, err
	}
	mq.ch, err = mq.conn.Channel()
	if err != nil {
		return nil, err
	}
	return mq, nil
}

func (mq *MQ) Listen() error {
	q, err := mq.ch.QueueDeclare(
		"cappa",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	msgs, err := mq.ch.Consume(
		q.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	for d := range msgs {
		data, err := Decode(d.Body)
		if err != nil {
			return err
		}
		fmt.Printf("Received a message: %s\n", data)
		Trigger(data.Id, data.Payload)
		d.Ack(false)
	}

	return nil
}

func (mq *MQ) Send(data Data) error {
	q, err := mq.ch.QueueDeclare(
		"cappa",
		true,
		false,
		false,
		false,
		nil,
	)
	body, err := data.Encode()
	if err != nil {
		return err
	}
	err = mq.ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         body,
		})
	if err != nil {
		return err
	}
	return nil
}

type NoTaskError struct{}

func (e *NoTaskError) Error() string {
	return fmt.Sprintf("Task doesn't exist")
}

func Trigger(task string, payload []byte) error {
	image := redis.Get("tasks", task)
	if image == "" {
		return &NoTaskError{}
	}
	event, err := uuid.NewV4()
	if err != nil {
		return err
	}
	eid := event.String()

	// Mount a file if specified
	pwd, err := os.Getwd()
	env := []string{}
	if payload != nil {
		filename := "payload"
		os.Mkdir(pwd+"/tmp", 0755)
		os.Mkdir(pwd+"/tmp/"+eid, 0755)
		out, err := os.Create(pwd + "/tmp/" + eid + "/" + filename)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = out.Write(payload)
		if err != nil {
			return err
		}
		env = append(env, "PAYLOAD=/payload/"+filename)
	}

	// Create and start the container
	cid := docker.Create(image, []string{pwd + "/tmp/" + eid + ":/payload"}, env)
	docker.Start(cid)

	// need a routine to update redis
	var logs string
	for i := 1; i < 100; i++ {
		logs = docker.Logs(cid)
	}
	fmt.Println(logs)
	redis.Set("logs", eid, logs)
	fmt.Println(eid)

	return nil
}