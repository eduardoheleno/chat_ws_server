package util

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PublishJob struct {
	Body []byte
	Queue string
}

func StartPublisherPool(conn *amqp.Connection, workers int) chan PublishJob {
	jobs := make(chan PublishJob, 1000)

	for i := range workers {
		go func (id int) {
			ch, err := conn.Channel()
			if err != nil {
				log.Fatalf("worker %d: failed to open channel: %v", id, err)
			}
			defer ch.Close()

			for job := range jobs {
				rabbitErr := ch.Publish(
					"",
					job.Queue,
					false,
					false,
					amqp.Publishing{
						DeliveryMode: amqp.Persistent,
						ContentType: "application/json",
						Body: job.Body,
					},
				)
				if rabbitErr != nil {
					log.Fatalf("Worker %d error: %s", id, rabbitErr)
				}
			}
		}(i)
	}

	return jobs
}
