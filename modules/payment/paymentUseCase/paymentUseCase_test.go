package paymentUsecase

import (
	"context"
	"errors"
	"testing"

	itemPb "github.com/watcharaphong99/InwzaShop/modules/item/itemPb"
	"github.com/watcharaphong99/InwzaShop/modules/payment"
)

type fakePaymentRepo struct {
	findItemsFn func(ctx context.Context, grpcUrl string, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error)
}

func (f *fakePaymentRepo) GetOffset(context.Context) (int64, error) {
	return -1, nil
}

func (f *fakePaymentRepo) UpsertOffset(context.Context, int64) error {
	return nil
}

func (f *fakePaymentRepo) FindItemsInIds(ctx context.Context, grpcUrl string, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
	return f.findItemsFn(ctx, grpcUrl, req)
}

func TestFindItemsInIds(t *testing.T) {
	t.Run("fills prices from item service", func(t *testing.T) {
		repo := &fakePaymentRepo{findItemsFn: func(_ context.Context, url string, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
			if url != "item-grpc" {
				t.Fatalf("grpc url = %q", url)
			}
			if len(req.Ids) != 1 || req.Ids[0] != "item:1" {
				t.Fatalf("ids = %v", req.Ids)
			}
			return &itemPb.FindItemsInIdsRes{Items: []*itemPb.Item{{Id: "item:1", Price: 50}}}, nil
		}}
		req := []*payment.ItemServiceReqDatum{{ItemId: "item:1"}, {ItemId: "item:1"}}
		if err := NewPaymentUsecase(repo).FindItemsInIds(context.Background(), "item-grpc", req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req[0].Price != 50 || req[1].Price != 50 {
			t.Fatalf("prices = (%v, %v), want 50", req[0].Price, req[1].Price)
		}
	})

	t.Run("propagates item service error", func(t *testing.T) {
		repo := &fakePaymentRepo{findItemsFn: func(context.Context, string, *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
			return nil, errors.New("error: item service is unavailable")
		}}
		err := NewPaymentUsecase(repo).FindItemsInIds(context.Background(), "item-grpc", []*payment.ItemServiceReqDatum{{ItemId: "item:1"}})
		if err == nil || err.Error() != "error: item service is unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("item not found", func(t *testing.T) {
		repo := &fakePaymentRepo{findItemsFn: func(context.Context, string, *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
			return &itemPb.FindItemsInIdsRes{Items: []*itemPb.Item{}}, nil
		}}
		err := NewPaymentUsecase(repo).FindItemsInIds(context.Background(), "item-grpc", []*payment.ItemServiceReqDatum{{ItemId: "item:missing"}})
		if err == nil || err.Error() != "error: items not found" {
			t.Fatalf("err = %v", err)
		}
	})
}
