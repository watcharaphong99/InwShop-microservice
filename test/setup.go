package test

import "github.com/watcharaphong99/InwzaShop/config"

func NewTestConfig() *config.Config {
	cfg := config.LoadConfig("../env/test/.env")
	return &cfg
}
