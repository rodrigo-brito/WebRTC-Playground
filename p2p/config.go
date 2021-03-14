package main

import (
	_ "embed"
	"log"

	"github.com/spf13/viper"
)

func init() {
	viper.AutomaticEnv()
	viper.AddConfigPath(".")
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatal(err)
		}
	}
	viper.SetDefault("p2p_port", 9000)
}

func TLSKey() string {
	return viper.GetString("tls_key")
}

func TLSCert() string {
	return viper.GetString("tls_cert")
}

func Port() int {
	return viper.GetInt("p2p_port")
}
