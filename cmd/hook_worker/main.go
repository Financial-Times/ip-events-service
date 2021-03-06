package main

import (
	"context"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/financial-times/ip-events-service/config"
	"github.com/financial-times/ip-events-service/consumer"
	"github.com/financial-times/ip-events-service/keen"
	"github.com/financial-times/ip-events-service/queue"
	"github.com/financial-times/ip-events-service/spoor"
)

var configPath = flag.String("config", "config_dev.yaml", "path to yaml config")

func main() {
	flag.Parse()
	conf, err := config.NewConfig(*configPath)
	if err != nil {
		log.Fatalln(err)
	}

	var writer io.Writer
	var msgChan chan queue.Message

	ctx, done := context.WithCancel(context.Background())

	switch conf.GOENV {
	case "production":
		msgChan = make(chan queue.Message)
		spoorChan := make(chan queue.Message)
		go func() {
			sc := spoor.NewClient(conf.SpoorHost)
			kc := keen.NewClient(conf.KeenProjectID, conf.KeenWriteKey)
			consumer.Consume(spoorChan, sc, kc)
			done()
		}()
		go func() {
			for {
				msg := <-msgChan
				spoorChan <- msg
			}
		}()
	case "staging":
		writer = ioutil.Discard
		msgChan = queue.Write(writer)
	default:
		msgChan = queue.Write(os.Stdout)
	}

	go func() {
		queue.Consume(queue.Redial(ctx, conf.RabbitHost, conf.QueueName), msgChan, conf.QueueName)
		done()
	}()
	<-ctx.Done()
}
