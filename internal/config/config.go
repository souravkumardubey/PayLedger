package config

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  int
	WriteTimeout int
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         "8080",
			ReadTimeout:  10,
			WriteTimeout: 10,
		},
		Database: DatabaseConfig{
			Driver: "postgres",
			DSN:    "postgres://postgres:postgres@localhost:5432/transaction_engine?sslmode=disable",
		},
	}
}
