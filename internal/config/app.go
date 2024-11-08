package config

type App struct {
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	Prometheus  Prometheus
	Health      Health
	Nats        Nats
	Push        Push
	DB          DB
	InternalAPI API
	Core        Core
}
