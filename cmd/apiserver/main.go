package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	_ "github.com/vega-project/ccb-operator/cmd/apiserver/docs"

	"github.com/vega-project/ccb-operator/pkg/grpc"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	port      int
	namespace string

	grpcClientOptions grpc.Options
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.IntVar(&o.port, "port", 8080, "Port number where the server will listen to")
	fs.StringVar(&o.namespace, "namespace", "vega", "The namespace where the calculations exist.")
	o.grpcClientOptions.Bind(fs)

	if err := fs.Parse(os.Args[1:]); err != nil {
		logrus.WithError(err).Fatal("couldn't parse arguments")
	}
	return o
}

func main() {
	o := gatherOptions()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clusterConfig, err := util.LoadClusterConfig()
	if err != nil {
		logrus.WithError(err).Fatal("could not load cluster clusterConfig")
	}

	c, err := ctrlruntimeclient.New(clusterConfig, ctrlruntimeclient.Options{})
	if err != nil {
		logrus.WithError(err).Fatal("failed to create client")
	}

	grpcClient, err := grpc.NewClient(o.grpcClientOptions.Address())
	if err != nil {
		logrus.WithError(err).Fatal("failed to construct grpc client")
	}

	s := server{
		logger:     logrus.WithField("component", "apiserver"),
		ctx:        ctx,
		client:     c,
		namespace:  o.namespace,
		grpcClient: grpcClient,
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/calculations", s.getCalculations)
	r.DELETE("/calculations/delete/:id", s.deleteCalculation)

	r.GET("/calculation", s.getCalculation)
	r.GET("/calculation/:id", s.getCalculationByName)

	r.GET("/bulks", s.getCalculationBulks)
	r.GET("/bulk/:id", s.getCalculationBulkByName)

	r.POST("/bulk/create", s.createCalculationBulk)
	r.DELETE("/bulks/delete/:id", s.deleteCalculationBulk)

	r.GET("/workerpools", s.getWorkerPools)
	r.GET("/workerpool/:id", s.getWorkerPoolByName)
	r.POST("workerpool/create", s.createWorkerPool)
	r.DELETE("/workerpools/delete/:id", s.deleteWorkerPool)
	r.POST("/results", s.getResults)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "OK"})
	})

	logrus.Infof("Listening on %d port", o.port)
	logrus.Fatal(r.Run(fmt.Sprintf(":%d", o.port)))
}
