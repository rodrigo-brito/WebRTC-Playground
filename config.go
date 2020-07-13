package main

import (
	"os/exec"

	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
)

var rootDir = "."

func init() {
	if b, err := exec.Command("go", "list", "-json", "-m").Output(); err == nil {
		rootDir = gjson.ParseBytes(b).Get("Dir").String() + "/"
	}

	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(rootDir)

	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(err)
		}
	}

	viper.SetDefault("port", 8000)
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
