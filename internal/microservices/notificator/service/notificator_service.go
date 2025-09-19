package service

import (
	"log"

	"wheres-my-pizza/internal/connections/rabbitmq"
)

type NotificatorService struct {
	rmqClient rabbitmq.Client
}

func NewNotificatorService(rqmClient rabbitmq.Client) *NotificatorService {
	return &NotificatorService{rmqClient: rqmClient}
}

func (ns *NotificatorService) Notify() {
	rmqChannel := ns.rmqClient.Channel()
	msgChannel, err := rmqChannel.Consume(
		"notifications_queue",
		"notificator",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println(err.Error())
		return
	}
	for message := range msgChannel {
		log.Println("Message received:", message.Body)
	}
}
