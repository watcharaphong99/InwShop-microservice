package playerHandler

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/IBM/sarama"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/player"
	playerUsecase "github.com/watcharaphong99/InwzaShop/modules/player/playerUseCase"
	queue "github.com/watcharaphong99/InwzaShop/pkg/kafka.go"
)

type (
	PlayerQueueHandlerService interface {
		Listen()
	}

	playerQueueHandler struct {
		cfg           *config.Config
		playerUsecase playerUsecase.PlayerUsecaseService
	}
)

func NewPlayerQueueHandler(cfg *config.Config, playerUsecase playerUsecase.PlayerUsecaseService) PlayerQueueHandlerService {
	return &playerQueueHandler{
		cfg:           cfg,
		playerUsecase: playerUsecase,
	}
}

func (h *playerQueueHandler) Listen() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	worker, err := queue.ConnectConsumer([]string{h.cfg.Kafka.Url}, h.cfg.Kafka.ApiKey, h.cfg.Kafka.Secret)
	if err != nil {
		log.Printf("Error: player consumer connect: %s", err.Error())
		return
	}
	defer worker.Close()

	offset, err := h.playerUsecase.GetOffset(ctx)
	if err != nil {
		log.Printf("Error: player offset: %s", err.Error())
		return
	}

	consumer, err := queue.ConsumeFromStoredOffset(worker, "player", offset)
	if err != nil {
		log.Printf("Error: player consumer: %s", err.Error())
		return
	}
	defer consumer.Close()

	log.Println("Start player consumer")
	for {
		select {
		case <-ctx.Done():
			log.Println("Stop player consumer")
			return
		case err, ok := <-consumer.Errors():
			if ok && err != nil {
				log.Printf("Error: player consumer: %s", err.Error())
			}
		case msg, ok := <-consumer.Messages():
			if !ok {
				return
			}
			h.handle(ctx, msg)
			if err := h.playerUsecase.UpsertOffset(ctx, msg.Offset+1); err != nil {
				log.Printf("Error: player offset: %s", err.Error())
			}
		}
	}
}

func (h *playerQueueHandler) handle(ctx context.Context, msg *sarama.ConsumerMessage) {
	switch string(msg.Key) {
	case "buy":
		req := new(player.CreatePlayerTransactionReq)
		if err := queue.DecodeMessage(req, msg.Value); err != nil {
			log.Printf("Error: player buy decode offset %d: %s", msg.Offset, err.Error())
			return
		}
		h.playerUsecase.DockedPlayerMoneyRes(ctx, h.cfg, req)
	case "sell":
		req := new(player.CreatePlayerTransactionReq)
		if err := queue.DecodeMessage(req, msg.Value); err != nil {
			log.Printf("Error: player sell decode offset %d: %s", msg.Offset, err.Error())
			return
		}
		h.playerUsecase.AddPlayerMoneyRes(ctx, h.cfg, req)
	case "rtransaction":
		req := new(player.RollbackPlayerTransactionReq)
		if err := queue.DecodeMessage(req, msg.Value); err != nil {
			log.Printf("Error: player rollback decode offset %d: %s", msg.Offset, err.Error())
			return
		}
		h.playerUsecase.RollbackPlayerTransaction(ctx, req)
	default:
		log.Printf("Info: player skip key %s offset %d", string(msg.Key), msg.Offset)
	}
}
