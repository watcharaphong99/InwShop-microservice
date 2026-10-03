package paymentRepository

import (
	"context"
	"errors"
	"log"
	"time"

	itemPb "github.com/watcharaphong99/InwzaShop/modules/item/itemPb"
	"github.com/watcharaphong99/InwzaShop/modules/models"
	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	PaymentRepositoryService interface {
		GetOffset(pctx context.Context) (int64, error)
		UpsertOffset(pctx context.Context, offset int64) error
		FindItemsInIds(pctx context.Context, grpcUrl string, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error)
	}

	paymentRepository struct {
		db *mongo.Client
	}
)

func NewPaymentRepository(db *mongo.Client) PaymentRepositoryService {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) paymentDbConn() *mongo.Database {
	return r.db.Database("payment_db")
}

func (r *paymentRepository) GetOffset(pctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.paymentDbConn()
	col := db.Collection("payment_queue")

	result := new(models.KafkaOffset)
	if err := col.FindOne(ctx, bson.M{"_id": models.KafkaOffsetID}).Decode(result); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return -1, nil
		}
		log.Printf("Error: GetOffset failed: %s", err.Error())
		return -1, errors.New("error: GetOffset failed")
	}

	return result.Offset, nil
}

func (r *paymentRepository) UpsertOffset(pctx context.Context, offset int64) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.paymentDbConn()
	col := db.Collection("payment_queue")

	result, err := col.UpdateOne(ctx, bson.M{"_id": models.KafkaOffsetID}, bson.M{"$max": bson.M{"offset": offset}}, options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("Error: UpsertOffset failed: %s", err.Error())
		return errors.New("error: UpsertOffset failed")
	}
	log.Printf("Info: UpsertOffset result: %v", result)

	return nil
}

func (r *paymentRepository) FindItemsInIds(pctx context.Context, grpcUrl string, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
	ctx, cancel := context.WithTimeout(pctx, 30*time.Second)
	defer cancel()

	jwtauth.SetApiKeyInContext(&ctx)

	conn, err := grpccon.NewGrpcClient(grpcUrl)
	if err != nil {
		log.Printf("Error: gRPC connection failed: %s", err.Error())
		return nil, errors.New("error: item service is unavailable")
	}
	defer conn.Close()

	result, err := conn.Item().FindItemsInIds(ctx, req)
	if err != nil {
		log.Printf("Error: FindItemsInIds failed: %s", err.Error())
		return nil, errors.New(grpcItemError(err))
	}

	if result == nil {
		return &itemPb.FindItemsInIdsRes{Items: make([]*itemPb.Item, 0)}, nil
	}

	if result.Items == nil {
		result.Items = make([]*itemPb.Item, 0)
	}

	return result, nil
}

func grpcItemError(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return "error: find items in ids failed"
	}

	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return "error: item service is unavailable"
	case codes.Unauthenticated, codes.PermissionDenied:
		return "error: grpc authorization failed"
	default:
		return "error: find items in ids failed"
	}
}
