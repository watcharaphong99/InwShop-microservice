package inventoryHandler

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/IBM/sarama"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/inventory"
	inventoryUsecase "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryUseCase"
	queue "github.com/watcharaphong99/InwzaShop/pkg/kafka.go"
)

type (
	InventoryQueueHandlerService interface {
		Listen()
	}

	inventoryQueueHandler struct {
		cfg              *config.Config
		inventoryUsecase inventoryUsecase.InventoryUsecaseService
	}
)

func NewInventoryQueueHandler(cfg *config.Config, inventoryUsecase inventoryUsecase.InventoryUsecaseService) InventoryQueueHandlerService {
	return &inventoryQueueHandler{
		cfg:              cfg,
		inventoryUsecase: inventoryUsecase,
	}
}

func (h *inventoryQueueHandler) Listen() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	worker, err := queue.ConnectConsumer([]string{h.cfg.Kafka.Url}, h.cfg.Kafka.ApiKey, h.cfg.Kafka.Secret)
	if err != nil {
		log.Printf("Error: inventory consumer connect: %s", err.Error())
		return
	}
	defer worker.Close()

	offset, err := h.inventoryUsecase.GetOffset(ctx)
	if err != nil {
		log.Printf("Error: inventory offset: %s", err.Error())
		return
	}

	consumer, err := queue.ConsumeFromStoredOffset(worker, "inventory", offset)
	if err != nil {
		log.Printf("Error: inventory consumer: %s", err.Error())
		return
	}
	defer consumer.Close()

	log.Println("Start inventory consumer")
	for {
		select {
		case <-ctx.Done():
			log.Println("Stop inventory consumer")
			return
		case err, ok := <-consumer.Errors():
			if ok && err != nil {
				log.Printf("Error: inventory consumer: %s", err.Error())
			}
		case msg, ok := <-consumer.Messages():
			if !ok {
				return
			}
			h.handle(ctx, msg)
			if err := h.inventoryUsecase.UpsertOffset(ctx, msg.Offset+1); err != nil {
				log.Printf("Error: inventory offset: %s", err.Error())
			}
		}
	}
}

func (h *inventoryQueueHandler) handle(ctx context.Context, msg *sarama.ConsumerMessage) {
	switch string(msg.Key) {
	case "buy":
		req := new(inventory.UpdateInventoryReq)
		if err := queue.DecodeMessage(req, msg.Value); err != nil {
			log.Printf("Error: inventory buy decode offset %d: %s", msg.Offset, err.Error())
			return
		}
		h.inventoryUsecase.AddPlayerItemRes(ctx, h.cfg, req)
	case "radd":
		req := new(inventory.RollbackPlayerInventoryReq)
		if err := queue.DecodeMessage(req, msg.Value); err != nil {
			log.Printf("Error: inventory radd decode offset %d: %s", msg.Offset, err.Error())
			return
		}
		h.inventoryUsecase.RollbackAddPlayerItem(ctx, h.cfg, req)
	case "sell":
		req := new(inventory.UpdateInventoryReq)
		if err := queue.DecodeMessage(req, msg.Value); err != nil {
			log.Printf("Error: inventory sell decode offset %d: %s", msg.Offset, err.Error())
			return
		}
		h.inventoryUsecase.RemovePlayerItemRes(ctx, h.cfg, req)
	case "rremove":
		req := new(inventory.RollbackPlayerInventoryReq)
		if err := queue.DecodeMessage(req, msg.Value); err != nil {
			log.Printf("Error: inventory rremove decode offset %d: %s", msg.Offset, err.Error())
			return
		}
		h.inventoryUsecase.RollbackRemovePlayerItem(ctx, h.cfg, req)
	default:
		log.Printf("Info: inventory skip key %s offset %d", string(msg.Key), msg.Offset)
	}
}
