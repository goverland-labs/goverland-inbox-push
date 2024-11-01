package main

import (
	"github.com/caarlos0/env/v6"
	_ "github.com/golang/mock/mockgen/model"
	"github.com/rs/zerolog"
	"github.com/s-larionov/process-manager"
	"github.com/shopspring/decimal"

	"github.com/goverland-labs/goverland-inbox-push/internal"
	"github.com/goverland-labs/goverland-inbox-push/internal/config"
	"github.com/goverland-labs/goverland-inbox-push/internal/logger"
)

const decimalDivisionPrecision = 32

var (
	cfg config.App
)

func init() {
	decimal.DivisionPrecision = decimalDivisionPrecision
	err := env.Parse(&cfg)
	if err != nil {
		panic(err)
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	zerolog.SetGlobalLevel(level)
	process.SetLogger(&logger.ProcessManagerLogger{})
}

func main() {
	app, err := internal.NewApplication(cfg)
	if err != nil {
		panic(err)
	}

	app.Run()
}
