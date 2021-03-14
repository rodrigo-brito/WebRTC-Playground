package main

import (
	_ "embed"

	"github.com/spf13/viper"
)

func init() {
	viper.AutomaticEnv()
	viper.AddConfigPath(".")
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(err)
		}
	}

	viper.SetDefault("sfu_port", 8000)
}

func TLSKey() string {
	return viper.GetString("tls_key")
}

func TLSCert() string {
	return viper.GetString("tls_cert")
}

func Port() int {
	return viper.GetInt("sfu_port")
}
