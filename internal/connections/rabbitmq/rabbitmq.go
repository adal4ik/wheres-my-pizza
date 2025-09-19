package rabbitmq

import (
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"wheres-my-pizza/internal/config"
)

type Client struct {
	url string

	mu   sync.RWMutex
	conn *amqp.Connection

	heartbeat    time.Duration
	retryBackoff time.Duration
	maxBackoff   time.Duration
}

// Dial — совместимый враппер под твой main: принимает config.RabbitMQConfig.
func Dial(c config.RabbitMQConfig) (*Client, error) {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/",
		c.User, c.Password, c.Host, c.Port,
	)
	return New(url)
}

// New — создание клиента по AMQP URL (используется внутри Dial).
func New(url string) (*Client, error) {
	cl := &Client{
		url:          url,
		heartbeat:    10 * time.Second,
		retryBackoff: 500 * time.Millisecond,
		maxBackoff:   8 * time.Second,
	}
	if err := cl.connect(); err != nil {
		return nil, err
	}
	return cl, nil
}

// Close — аккуратно закрывает текущее соединение.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil && !c.conn.IsClosed() {
		return c.conn.Close()
	}
	return nil
}

// Channel — всегда возвращает рабочий канал; при необходимости переподключается.
func (c *Client) Channel() *amqp.Channel {
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil || conn.IsClosed() {
			c.reconnectLoop()
			continue
		}
		ch, err := conn.Channel()
		if err == nil {
			return ch
		}
		log.Printf(`{"level":"WARN","amqp":"channel_open_failed","err":%q}`, err.Error())
		c.reconnectLoop()
	}
}

// --- внутренняя кухня ---

func (c *Client) connect() error {
	cfg := amqp.Config{
		Heartbeat: c.heartbeat,
		Locale:    "en_US",
		Properties: amqp.Table{
			"connection_name": "wheres-my-pizza",
		},
	}
	conn, err := amqp.DialConfig(c.url, cfg)
	if err != nil {
		return fmt.Errorf("amqp dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	// мониторим закрытие
	go func() {
		if err := <-conn.NotifyClose(make(chan *amqp.Error, 1)); err != nil {
			log.Printf(`{"level":"ERROR","amqp":"connection_closed","code":%d,"reason":%q}`, err.Code, err.Reason)
		} else {
			log.Printf(`{"level":"WARN","amqp":"connection_closed","reason":"EOF"}`)
		}
		c.reconnectLoop()
	}()

	// блокировки/разблокировки (по желанию)
	go func() {
		for b := range conn.NotifyBlocked(make(chan amqp.Blocking, 1)) {
			if b.Active {
				log.Printf(`{"level":"WARN","amqp":"connection_blocked","reason":%q}`, b.Reason)
			} else {
				log.Printf(`{"level":"INFO","amqp":"connection_unblocked"}`)
			}
		}
	}()

	log.Printf(`{"level":"INFO","amqp":"connected"}`)
	return nil
}

func (c *Client) reconnectLoop() {
	backoff := c.retryBackoff
	for {
		c.mu.RLock()
		ok := c.conn != nil && !c.conn.IsClosed()
		c.mu.RUnlock()
		if ok {
			return
		}
		if err := c.connect(); err == nil {
			return
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > c.maxBackoff {
			backoff = c.maxBackoff
		}
	}
}
