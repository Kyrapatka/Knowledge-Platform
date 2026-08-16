package main

import (
	"log"

	"github.com/Kyrapatka/knowledge-platform/config"
	"github.com/Kyrapatka/knowledge-platform/internal/app"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(
			"load config: %v",
			err,
		)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf(
			"create application: %v",
			err,
		)
	}

	defer func() {
		if err := application.Close(); err != nil {
			log.Printf(
				"close application: %v",
				err,
			)
		}
	}()

	if err := application.Run(); err != nil {
		log.Fatalf(
			"run application: %v",
			err,
		)
	}
}
