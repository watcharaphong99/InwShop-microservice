package paymentUsecase

import (
	"context"
	"errors"
	"log"

	"github.com/IBM/sarama"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/item"
	itemPb "github.com/watcharaphong99/InwzaShop/modules/item/itemPb"
	"github.com/watcharaphong99/InwzaShop/modules/payment"
	"github.com/watcharaphong99/InwzaShop/modules/payment/paymentRepository"
	queue "github.com/watcharaphong99/InwzaShop/pkg/kafka.go"
)

type (
	PaymentUsecaseService interface {
		FindItemsInIds(pactx context.Context, grpcUrl string, req []*payment.ItemServiceReqDatum) error
		GetOffset(pctx context.Context) (int64, error)
		UpsertOffset(pctx context.Context, offset int64) error
	}

	paymentUsecase struct {
		paymentRepository paymentRepository.PaymentRepositoryService
	}
)

func NewPaymentUsecase(paymentRepository paymentRepository.PaymentRepositoryService) PaymentUsecaseService {
	return &paymentUsecase{paymentRepository: paymentRepository}
}

func (u *paymentUsecase) GetOffset(pctx context.Context) (int64, error) {
	return u.paymentRepository.GetOffset(pctx)
}

func (u *paymentUsecase) UpsertOffset(pctx context.Context, offset int64) error {
	return u.paymentRepository.UpsertOffset(pctx, offset)
}

func (u *paymentUsecase) FindItemsInIds(pactx context.Context, grpcUrl string, req []*payment.ItemServiceReqDatum) error {
	setIds := make(map[string]bool)
	for _, v := range req {
		if !setIds[v.ItemId] {
			setIds[v.ItemId] = true
		}
	}

	itemData, err := u.paymentRepository.FindItemsInIds(pactx, grpcUrl, &itemPb.FindItemsInIdsReq{
		Ids: func() []string {
			itemIds := make([]string, 0)
			for k := range setIds {
				itemIds = append(itemIds, k)
			}
			return itemIds
		}(),
	})
	if err != nil {
		log.Printf("Error: FindItemsInIds failed: %s", err.Error())
		return err
	}

	itemMaps := make(map[string]*item.ItemShowCase)
	for _, v := range itemData.Items {
		itemMaps[v.Id] = &item.ItemShowCase{
			ItemId:   v.Id,
			Title:    v.Title,
			Price:    v.Price,
			ImageUrl: v.ImageUrl,
			Damage:   int(v.Damage),
		}
	}

	for i := range req {
		itemShowCase, ok := itemMaps[req[i].ItemId]
		if !ok {
			log.Printf("Error: FindItemsInIds: item %s not found", req[i].ItemId)
			return errors.New("error: items not found")
		}
		req[i].Price = itemShowCase.Price
	}

	return nil
}

func (u *paymentUsecase) PaymentConsumer(ctx context.Context, cfg *config.Config) (sarama.PartitionConsumer, error) {
	worker, err := queue.ConnectConsumer([]string{cfg.Kafka.Url}, cfg.Kafka.ApiKey, cfg.Kafka.Secret)
	if err != nil {
		return nil, err
	}

	offset, err := u.paymentRepository.GetOffset(ctx)
	if err != nil {
		return nil, err
	}

	consumer, err := worker.ConsumePartition("payment", 0, offset)
	if err != nil {
		log.Println("Trying to set offset as 0")
		consumer, err = worker.ConsumePartition("payment", 0, 0)
		if err != nil {
			log.Println("Error: PaymentConsumer failed: ", err.Error())
			return nil, nil
		}
	}
	return consumer, nil
}

func (u *paymentUsecase) BuyOrSellConsumer(pctx context.Context, key string, cfg *config.Config, resCh chan<- *payment.PaymentTransferRes) {
}

func (u *paymentUsecase) BuyItem(pctx context.Context, cfg *config.Config, playerId string, req *payment.ItemServiceReq) (*payment.PaymentTransferRes, error) {
	if err := u.FindItemsInIds(pctx, cfg.Grpc.ItemUrl, req.Items); err != nil {
		return nil, nil
	}
	return nil, nil
}

func (u *paymentUsecase) SellItem(pctx context.Context, cfg *config.Config, playerId string, req *payment.ItemServiceReq) (*payment.PaymentTransferRes, error) {
	if err := u.FindItemsInIds(pctx, cfg.Grpc.ItemUrl, req.Items); err != nil {
		return nil, nil
	}
	return nil, nil
}
