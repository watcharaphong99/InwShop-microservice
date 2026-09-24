package main

import (
	"context"
	"log"
	"os"

	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/pkg/database"
	"github.com/watcharaphong99/InwzaShop/pkg/rediscon"
	"github.com/watcharaphong99/InwzaShop/server"
)

func main() {
	ctx := context.Background()
	_ = ctx

	//initialize Config()
	cfg := config.LoadConfig(func() string {
		println("os.args", os.Args)
		if len(os.Args) < 2 {
			log.Fatal("Error : .env path is required")
		}
		return os.Args[1]
	}())

	//database connect
	db := database.DbConn(ctx, &cfg)
	cache := rediscon.NewClient(cfg.Redis.Url)
	defer db.Disconnect(ctx)
	// log.Println(db)

	//server start
	server.Start(ctx, &cfg, db, cache)
}
