package grpc

import "flag"

type Options struct {
	address string
}

func (o *Options) Bind(fs *flag.FlagSet) {
	fs.StringVar(&o.address, "grpc-address", "results-handler:50051", "gRPC server address")
}

func (o *Options) Address() string {
	return o.address
}
