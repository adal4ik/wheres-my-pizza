package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/kitchen/repository"
)

var (
	ErrRequeue = errors.New("requeue")     // nack(requeue=true)
	ErrDLQ     = errors.New("dead_letter") // nack(requeue=false)
)

type KitchenServiceInterface interface {
	Heartbeat(ctx context.Context) error
	Run(ctx context.Context) error
}

type KitchenService struct {
	db        repository.KitchenRepositoryInterface
	rmqClient rabbitmq.Client // у тебя клиент по значению — оставляю так

	WorkerName string
	WorkerType string

	Queue       string        // "orders_queue"
	OrderTypes  []string      // специализация (если пусто — любые)
	Prefetch    int           // QoS (для последовательности держим =1)
	BeatEvery   time.Duration // heartbeat interval
	UseConfirms bool          // publisher confirms (опц.)

	// cooking times
	CookDineIn   time.Duration
	CookTakeout  time.Duration
	CookDelivery time.Duration

	pubAcks <-chan amqp091.Confirmation
}

func NewKitchenService(db repository.KitchenRepositoryInterface, rmqClient rabbitmq.Client, workerName string, orderTypesCSV string, prefetch int, heartbeatInterval int) KitchenServiceInterface {
	types := make([]string, 0)
	if s := strings.TrimSpace(orderTypesCSV); s != "" {
		for _, t := range strings.Split(s, ",") {
			if tt := strings.TrimSpace(t); tt != "" {
				types = append(types, tt)
			}
		}
	}
	if prefetch <= 0 {
		prefetch = 1
	}
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30
	}
	return &KitchenService{
		db:           db,
		rmqClient:    rmqClient,
		WorkerName:   workerName,
		WorkerType:   "generic",
		Queue:        "orders_queue",
		OrderTypes:   types,
		Prefetch:     prefetch, // оставь 1 для строго последовательной обработки
		BeatEvery:    time.Duration(heartbeatInterval) * time.Second,
		UseConfirms:  false,
		CookDineIn:   8 * time.Second,
		CookTakeout:  10 * time.Second,
		CookDelivery: 12 * time.Second,
	}
}

func (ks *KitchenService) Heartbeat(ctx context.Context) error {
	return ks.db.Heartbeat(ctx, ks.WorkerName)
}

func (ks *KitchenService) Run(ctx context.Context) error {
	rmqChannel := ks.rmqClient.Channel()
	err := rmqChannel.Qos(ks.Prefetch, 0, false)
	if err != nil {
		return err
	}
	defer rmqChannel.Close()
	messageChannel, err := rmqChannel.Consume(
		ks.Queue,
		ks.WorkerName,
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // noWait
	)
	for err != nil {
		fmt.Println(err.Error())
		return err
	}
	for msg := range messageChannel {
		log.Println("Received a message: ", string(msg.Body))
	}
	return nil
}
