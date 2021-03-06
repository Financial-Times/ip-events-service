package queue

import (
	"log"

	"github.com/streadway/amqp"
)

// Message type for rabbitmq
type Message struct {
	Body     []byte
	Response chan bool
}

// Publish publishes message to a queue
func Publish(sessions chan chan Session, msgs <-chan Message, queueName string) {

	for session := range sessions {
		var (
			running bool
			reading = msgs
			pending = make(chan Message, 1)
			confirm = make(chan amqp.Confirmation, 1)
		)

		pub := <-session

		// publisher confirms for this channel/connection
		if err := pub.Confirm(false); err != nil {
			log.Printf("publisher confirms not supported")
			close(confirm) // confirms not supported, simulate by always nacking
		} else {
			pub.NotifyPublish(confirm)
		}

	Publish:
		for {
			var body Message
			select {
			case confirmed, ok := <-confirm:
				if !ok {
					break Publish
				}
				if !confirmed.Ack {
					log.Printf("nack message %d, body: %q", confirmed.DeliveryTag, string(body.Body))
				}
				reading = msgs

			case body = <-pending:
				err := pub.Publish("", queueName, false, false, amqp.Publishing{
					Body: body.Body,
				})

				// Retry failed delivery on next session
				if err != nil {
					body.Response <- false
					pending <- body
					pub.Close()
					break Publish
				}

				// Let response channel know all is ok
				body.Response <- true

			case body, running = <-reading:
				// all messages consumed
				if !running {
					return
				}
				// work on pending delivery until ack'd
				pending <- body
				reading = nil
			}
		}
	}
}
