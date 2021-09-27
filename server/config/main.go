package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

var config *viper.Viper

// Init is an exported method that takes the environment starts the viper
// (external lib) and returns the configuration struct.
func Init(env string) {
	config = viper.New()
	config.SetConfigType("yaml")
	config.SetConfigName("config")
	config.AddConfigPath(".")
	config.AddConfigPath("/etc/")
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	config.AutomaticEnv()
	if err := config.ReadInConfig(); err != nil {
		log.Println("WARNING: could not load configuration file. Using ENV variables")
	}
}

func Config() *viper.Viper {
	return config
}
