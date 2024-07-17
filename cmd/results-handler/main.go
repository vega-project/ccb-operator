package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	"github.com/sirupsen/logrus"

	"github.com/vega-project/ccb-operator/pkg/db"
	pb "github.com/vega-project/ccb-operator/pkg/db/proto"
	grpc_server "github.com/vega-project/ccb-operator/pkg/grpcserver"
)

type options struct {
	port            int
	databaseOptions db.Options
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.IntVar(&o.port, "port", 50051, "Port number where the gRPC server will listen to")
	o.databaseOptions.Bind(fs)

	if err := fs.Parse(os.Args[1:]); err != nil {
		logrus.WithError(err).Fatal("couldn't parse arguments.")
	}
	return o
}

func validateOptions(o options) error {
	return o.databaseOptions.Validate()
}

func main() {
	o := gatherOptions()
	err := validateOptions(o)
	if err != nil {
		logrus.WithError(err).Fatal("invalid options")
	}

	resultstore, err := o.databaseOptions.NewCalculationResultsStore()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't initialize database")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", o.port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterDbServiceServer(s, grpc_server.NewServer(resultstore))

	log.Printf("Server listening on port %d", o.port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
