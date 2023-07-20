package config

type API struct {
	InboxStorageAddress string `env:"INTERNAL_API_INBOX_STORAGE_ADDRESS" envDefault:"localhost:11100"`
}
