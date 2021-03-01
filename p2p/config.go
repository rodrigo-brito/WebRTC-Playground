package main

import (
	_ "embed"
	"log"
	"strings"

	"github.com/spf13/viper"
)

//go:embed .env
var configFile string

func init() {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	err := viper.ReadConfig(strings.NewReader(configFile))
	if err != nil {
		log.Fatal(err)
	}

	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(err)
		}
	}

	viper.SetDefault("port", 9000)
	viper.SetDefault("tls_key", "key.pem")
	viper.SetDefault("tls_cert", "cert.pem")
}

func TLSKey() string {
	return viper.GetString("tls_key")
}

func TLSCert() string {
	return viper.GetString("tls_cert")
}

func Port() int {
	return viper.GetInt("port")
}
