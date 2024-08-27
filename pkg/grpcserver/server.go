package grpc_server

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/vega-project/ccb-operator/pkg/db"
	proto "github.com/vega-project/ccb-operator/proto"
)

type Server struct {
	resultstore db.CalculationResultsStore
	proto.UnimplementedDbServiceServer
}

func NewServer(resultstore db.CalculationResultsStore) *Server {
	return &Server{resultstore: resultstore}
}

func (s *Server) StoreData(ctx context.Context, in *proto.StoreRequest) (*proto.StoreReply, error) {
	l := logrus.WithField("parametres", in.Parameters)
	reply, err := s.resultstore.StoreOrUpdateData(ctx, in)
	if err != nil {
		l.WithError(err).Error("error storing or updating data")
	} else {
		l.Infof(reply.GetMessage())
	}
	return reply, err
}
