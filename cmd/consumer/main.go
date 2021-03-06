package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/robertke/kafka-consumer/pkg/infrastructure/config"
	"github.com/robertke/kafka-consumer/pkg/infrastructure/consumer"
	"github.com/robertke/kafka-consumer/pkg/infrastructure/db"
	"github.com/robertke/kafka-consumer/pkg/infrastructure/handler"
	logger "github.com/robertke/kafka-consumer/pkg/infrastructure/log"
	"github.com/robertke/kafka-consumer/pkg/repo"
)

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	stopCh := make(chan bool)

	l, err := logger.NewLogger(logger.DebugLogConfig)
	if err != nil {
		log.Fatalf("could not init logger: %v", err)
	}

	sqlDB, err := db.Connect(conf.DB.Driver, conf.DB)
	if err != nil {
		log.Fatalf("could not init db connection: %v", err)
	}

	if err := db.Migrate(conf.DB.MigrationsPath, sqlDB, l); err != nil {
		log.Fatalf("could run migrations: %v", err)
	}

	sugar := l.Sugar()

	c, err := consumer.New(&conf, l)
	if err != nil {
		sugar.Errorf("consumer error %v", err)
	}

	fooRepo := repo.NewFoo(sqlDB)
	h := handler.NewMsg(l, fooRepo)

	if er := c.Run(ctx, h); er != nil {
		sugar.Errorf("consumer run error %v", er)
	}

	signalListener(cancel)

	stopChListener(stopCh, ctx)

	sugar.Info("Running kafka consumer...")
	<-stopCh
}

func stopChListener(stopCh chan<- bool, ctx context.Context) {
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("Received interrupt signal, exiting...")
				stopCh <- true
				return
			default:
			}
		}
	}(ctx)
}

func signalListener(cancel context.CancelFunc) {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	go func() {
		select {
		case <-signalCh:
			cancel()
			return
		}
	}()
}
