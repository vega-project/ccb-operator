package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vega-project/ccb-operator/pkg/db"
	proto "github.com/vega-project/ccb-operator/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type Server struct {
	resultstore db.CalculationResultsStore
	proto.UnimplementedDbServiceServer
}

func NewServer(resultstore db.CalculationResultsStore) *Server {
	return &Server{resultstore: resultstore}
}

func (s *Server) StoreData(ctx context.Context, in *proto.StoreRequest) (*proto.StoreResponse, error) {
	l := logrus.WithField("parametres", in.Parameters)
	reply, err := s.resultstore.StoreOrUpdateData(ctx, in)
	if err != nil {
		l.WithError(err).Error("error storing or updating data")
		return nil, err
	}

	l.Info(reply.GetMessage())
	return reply, err
}

func (s *Server) GetData(ctx context.Context, in *proto.GetDataRequest) (*proto.GetDataResponse, error) {
	l := logrus.WithField("parametres", in.Parameters)
	reply, err := s.resultstore.GetData(ctx, in.Parameters)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		l.WithError(err).Error("error getting data")
		return nil, err
	}
	return &proto.GetDataResponse{
		Results:   reply.Results,
		CreatedAt: reply.CreatedAt.Format(time.RFC3339),
	}, err
}
