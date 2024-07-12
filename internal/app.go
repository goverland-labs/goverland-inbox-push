package internal

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	coresdk "github.com/goverland-labs/core-web-sdk"
	"github.com/goverland-labs/inbox-api/protobuf/inboxapi"
	"github.com/nats-io/nats.go"
	"github.com/s-larionov/process-manager"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goverland-labs/inbox-push/internal/config"
	"github.com/goverland-labs/inbox-push/internal/sender"
	"github.com/goverland-labs/inbox-push/pkg/health"
	"github.com/goverland-labs/inbox-push/pkg/prometheus"
)

type Application struct {
	sigChan <-chan os.Signal
	manager *process.Manager
	cfg     config.App
	db      *gorm.DB
}

func NewApplication(cfg config.App) (*Application, error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	a := &Application{
		sigChan: sigChan,
		cfg:     cfg,
		manager: process.NewManager(),
	}

	err := a.bootstrap()
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (a *Application) Run() {
	a.manager.StartAll()
	a.registerShutdown()
}

func (a *Application) bootstrap() error {
	initializers := []func() error{
		a.initDB,

		// Init Dependencies
		a.initServices,

		// Init Workers: Application
		// TODO

		// Init Workers: System
		a.initPrometheusWorker,
		a.initHealthWorker,
	}

	for _, initializer := range initializers {
		if err := initializer(); err != nil {
			return err
		}
	}

	return nil
}

func (a *Application) initDB() error {
	db, err := gorm.Open(postgres.Open(a.cfg.DB.DSN), &gorm.Config{})
	if err != nil {
		return err
	}

	ps, err := db.DB()
	if err != nil {
		return err
	}
	ps.SetMaxOpenConns(a.cfg.DB.MaxOpenConnections)

	a.db = db
	if a.cfg.DB.Debug {
		a.db = db.Debug()
	}

	return err
}

func (a *Application) initServices() error {
	nc, err := nats.Connect(
		a.cfg.Nats.URL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(a.cfg.Nats.MaxReconnects),
		nats.ReconnectWait(a.cfg.Nats.ReconnectTimeout),
	)
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(
		a.cfg.InternalAPI.InboxStorageAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("create connection with core storage server: %v", err)
	}

	subs := inboxapi.NewSubscriptionClient(conn)
	usrs := inboxapi.NewUserClient(conn)
	sp := inboxapi.NewSettingsClient(conn)
	coreSDK := coresdk.NewClient(a.cfg.Core.CoreURL)

	repo := sender.NewRepo(a.db)
	service, err := sender.NewService(repo, a.cfg.Push, subs, usrs, sp, coreSDK)
	if err != nil {
		return err
	}

	dc, err := sender.NewConsumer(nc, service)
	if err != nil {
		return fmt.Errorf("sender consumer: %w", err)
	}

	postman := sender.NewPostmanWorker(service)

	a.manager.AddWorker(process.NewCallbackWorker("sender-consumer", dc.Start))
	a.manager.AddWorker(process.NewCallbackWorker("postman-immediately", postman.StartImmediately))
	a.manager.AddWorker(process.NewCallbackWorker("postman-regular", postman.StartRegular))

	return nil
}

func (a *Application) initPrometheusWorker() error {
	srv := prometheus.NewServer(a.cfg.Prometheus.Listen, "/metrics")
	a.manager.AddWorker(process.NewServerWorker("prometheus", srv))

	return nil
}

func (a *Application) initHealthWorker() error {
	srv := health.NewHealthCheckServer(a.cfg.Health.Listen, "/status", health.DefaultHandler(a.manager))
	a.manager.AddWorker(process.NewServerWorker("health", srv))

	return nil
}

func (a *Application) registerShutdown() {
	go func(manager *process.Manager) {
		<-a.sigChan

		manager.StopAll()
	}(a.manager)

	a.manager.AwaitAll()
}
