package paymentHandler

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/payment"
	paymentUsecase "github.com/watcharaphong99/InwzaShop/modules/payment/paymentUseCase"
	queue "github.com/watcharaphong99/InwzaShop/pkg/kafka.go"
)

type (
	PaymentQueueHandlerService interface {
		Listen()
	}

	paymentQueueHandler struct {
		config         *config.Config
		paymentUsecase paymentUsecase.PaymentUsecaseService
	}
)

func NewPaymentQueue(config *config.Config, paymentUsecase paymentUsecase.PaymentUsecaseService) PaymentQueueHandlerService {
	return &paymentQueueHandler{
		config:         config,
		paymentUsecase: paymentUsecase,
	}
}

func (h *paymentQueueHandler) Listen() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	worker, err := queue.ConnectConsumer([]string{h.config.Kafka.Url}, h.config.Kafka.ApiKey, h.config.Kafka.Secret)
	if err != nil {
		log.Printf("Error: payment consumer connect: %s", err.Error())
		return
	}
	defer worker.Close()

	offset, err := h.paymentUsecase.GetOffset(ctx)
	if err != nil {
		log.Printf("Error: payment offset: %s", err.Error())
		return
	}

	consumer, err := queue.ConsumeFromStoredOffset(worker, "payment", offset)
	if err != nil {
		log.Printf("Error: payment consumer: %s", err.Error())
		return
	}
	defer consumer.Close()

	log.Println("Start payment reply consumer")
	// รัน recover saga ค้างทันทีตอนสตาร์ท แล้ววนซ้ำตาม interval (ไม่พึ่ง restart อย่างเดียว)
	h.paymentUsecase.RecoverStaleOrderWorkflows(ctx, h.config)
	go h.runStaleOrderWorkflowRecoveryLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stop payment reply consumer")
			return
		case err, ok := <-consumer.Errors():
			if ok && err != nil {
				log.Printf("Error: payment consumer: %s", err.Error())
			}
		case msg, ok := <-consumer.Messages():
			if !ok {
				return
			}
			h.accept(ctx, msg)
		}
	}
}

func (h *paymentQueueHandler) runStaleOrderWorkflowRecoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(paymentUsecase.OrderWorkflowRecoverInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.paymentUsecase.RecoverStaleOrderWorkflows(ctx, h.config)
		}
	}
}

func (h *paymentQueueHandler) accept(ctx context.Context, msg *sarama.ConsumerMessage) {
	res := new(payment.PaymentTransferRes)
	if err := queue.DecodeMessage(res, msg.Value); err != nil {
		log.Printf("Error: payment reply decode offset %d: %s", msg.Offset, err.Error())
	} else {
		h.paymentUsecase.AcceptPaymentReply(res)
	}

	if err := h.paymentUsecase.UpserOffset(ctx, msg.Offset+1); err != nil {
		log.Printf("Error: payment offset: %s", err.Error())
	}
}
