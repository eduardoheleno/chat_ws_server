package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"nossochat_rt_service/util"
	"strconv"
	"sync"
	"time"

	"jwt_auth"

	"github.com/gorilla/websocket"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	userId string
	conn *websocket.Conn
	ch chan []byte
}

type WsMessage struct {
	SenderId uint `json:"sender_id"`
	ReceiverId uint `json:"receiver_id"`
	ChatId uint `json:"chat_id"`
	ReceiverEmail string `json:"receiver_email"`

	Nonce []byte `json:"nonce"`
	Content []byte `json:"content"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	nodeHash string
	defaultCtx = context.Background()
	redisClient *redis.Client
	amqpConn *amqp.Connection
	publishJobCh chan util.PublishJob
	clients = make(map[string]*Client)
	mu sync.RWMutex
)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	jwtToken := r.Header.Get("authToken")
	jwtClaims := jwt_auth.ValidateToken(jwtToken)
	if jwtClaims == nil {
		util.PreventHandshakeOnError("Not valid token", w)
		return
	}

	userId := strconv.FormatUint(uint64(jwtClaims.UserId), 10)

	setErr := redisClient.Set(defaultCtx, userId, nodeHash, time.Second * 30).Err()
	if setErr != nil {
		util.PreventHandshakeOnError("Redis error", w)
		log.Printf("Can't store userId -> nodeHash")
		return
	}

	// TODO: store array of contacts to check if message has a valid target user
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		util.PreventHandshakeOnError("Error on upgrading connection", w)
		log.Printf("Connection upgrade error: %s", err)
		return
	}

	client := &Client{
		userId: userId,
		conn: conn,
		ch: make(chan []byte, 64),
	}

	mu.Lock()
	clients[userId] = client
	mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	go readConn(client, cancel)
	go watchClientChannel(client, ctx)
}

func watchClientChannel(client *Client, ctx context.Context) {
	defer client.conn.Close()
	defer func() {
		redisClient.Del(defaultCtx, client.userId)

		mu.Lock()
		delete(clients, client.userId)
		mu.Unlock()
	}()

	for {
		select {
		case msg := <- client.ch:
			client.conn.WriteMessage(1, msg)
		case <- ctx.Done():
			return
		}
	}
}

func readConn(client *Client, cancel context.CancelFunc) {
	defer cancel()

	for {
		messageType, message, err := client.conn.ReadMessage()
		if messageType < 0 {
			break;
		}
		if err != nil {
			fmt.Println("Error reading message:", err)
			break;
		}

		job := util.PublishJob{
			Queue: "messages",
			Body: message,
		}

		publishJobCh <- job
	}
}

func updateClientsTTL() {
	ticker := time.NewTicker(time.Second * 15)

	for range ticker.C {
		mu.RLock()
		pipe := redisClient.Pipeline()

		for clientId := range clients {
			pipe.Expire(defaultCtx, clientId, time.Second * 30)
		}

		_, err := pipe.Exec(defaultCtx)
		if err != nil {
			log.Fatalf("Can't update clients TTL: %s\n", err)
		} else {
			log.Println("TLL's updated")
		}

		mu.RUnlock()
	}
}

func consumeNodeMessages() {
	ch, err := amqpConn.Channel()
	util.FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		nodeHash,
		true,
		false,
		true,
		false,
		nil,
	)
	util.FailOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name,
		"",
		false,
		true,
		false,
		false,
		nil,
	)
	util.FailOnError(err, "Failed to register a consumer")

	var forever chan struct{}
	go func() {
		for d := range msgs {
			var parsedMsg map[string]json.RawMessage
			err := json.Unmarshal(d.Body, &parsedMsg)
			if err != nil {
				d.Ack(false)
				log.Println(err.Error())
				continue
			}

			mu.RLock()
			client := clients[string(parsedMsg["target_id"])]
			if client == nil {
				d.Ack(false)
				mu.RUnlock()
				log.Println("User is not connected anymore")
				continue
			}

			client.ch <- d.Body
			mu.RUnlock()
			d.Ack(false)
		}
	}()
	log.Println("[*] Waiting for messages...")
	<-forever
}

func main() {
	nodeHash = util.GenerateNodeHash()

	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		Password: "",
		DB: 0,
	})

	var err error
	amqpConn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	util.FailOnError(err, "Fail to connect to RabbitMQ")
	defer amqpConn.Close()

	publishJobCh = util.StartPublisherPool(amqpConn, 5)
	go consumeNodeMessages()
	go updateClientsTTL()

	http.HandleFunc("/", wsHandler)
	log.Printf("Websocket server '%s' started on :8000\n", nodeHash)
	httpErr := http.ListenAndServe(":8000", nil)
	if httpErr != nil {
		log.Panicln("Error starting server:", httpErr)
	}
}
