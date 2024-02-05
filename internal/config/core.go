package config

type Core struct {
	CoreURL string `env:"CORE_URL" envDefault:"https://core.goverland.xyz/v1"`
}
