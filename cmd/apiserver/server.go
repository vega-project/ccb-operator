package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type server struct {
	logger      *logrus.Entry
	namespace   string
	ctx         context.Context
	client      ctrlruntimeclient.Client
	resultsPath string
}

func (s *server) createCalculationBulk(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		responseError(c, "couldn't read body", err)
		return
	}

	bulkName := fmt.Sprintf("bulk-%s", util.InputHash(body))
	var bulkCalcs struct {
		WorkerPool   string                        `json:"worker_pool,omitempty"`
		Calculations map[string]bulkv1.Calculation `json:"calculations,omitempty"`
	}

	if err := json.Unmarshal(body, &bulkCalcs); err != nil {
		responseError(c, "couldn't unmarshal body", err)
	}

	s.logger.Info("Creating calculation bulk...")
	bulk := &bulkv1.CalculationBulk{
		ObjectMeta:   metav1.ObjectMeta{Name: bulkName, Namespace: s.namespace},
		WorkerPool:   bulkCalcs.WorkerPool,
		Calculations: bulkCalcs.Calculations,
		Status:       bulkv1.CalculationBulkStatus{State: bulkv1.CalculationBulkAvailableState},
	}

	if err := s.client.Create(s.ctx, bulk); err != nil {
		responseError(c, "couldn't create calculation bulk", err)
	} else {
		c.JSON(http.StatusOK, gin.H{"data": bulk})
	}
}

func (s *server) createWorkerPool(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		responseError(c, "couldn't read body", err)
		return
	}

	workerPoolName := fmt.Sprintf("workerpool-%s", util.InputHash(body))
	var workerPools struct {
		Workers        map[string]workersv1.Worker `json:"workers,omitempty"`
		CompletionTime metav1.Time                 `json:"completionTime,omitempty"`
		CreationTime   metav1.Time                 `json:"creationTime,omitempty"`
		PendingTime    metav1.Time                 `json:"pendingTime,omitempty"`
	}

	if err := json.Unmarshal(body, &workerPools); err != nil {
		responseError(c, "couldn't unmarshal body", err)
	}

	s.logger.Info("Creating the workerpool...")

	wp := &workersv1.WorkerPool{
		ObjectMeta: metav1.ObjectMeta{Name: workerPoolName, Namespace: s.namespace},
		Spec: workersv1.WorkerPoolSpec{
			Workers: workerPools.Workers,
		},
		Status: workersv1.WorkerPoolStatus{
			CreationTime:   &workerPools.CreationTime,
			PendingTime:    &workerPools.PendingTime,
			CompletionTime: &workerPools.CompletionTime,
		},
	}

	if err := s.client.Create(s.ctx, wp); err != nil {
		responseError(c, "couldn't create calculation bulk", err)
	} else {
		c.JSON(http.StatusOK, gin.H{"data": wp})
	}
}

func (s *server) deleteCalculationBulk(c *gin.Context) {
	calcBulkName := c.Param("id")
	bulk := &bulkv1.CalculationBulk{}
	err := s.client.Get(s.ctx, ctrlruntimeclient.ObjectKey{Namespace: s.namespace, Name: calcBulkName}, bulk)
	if err != nil {
		responseError(c, fmt.Sprintf("failed to get the calculation bulk %s", calcBulkName), err)
	}

	if err := s.client.Delete(s.ctx, bulk); err != nil {
		responseError(c, fmt.Sprintf("failed to delete the calculation bulk %s", calcBulkName), err)
	} else {
		c.JSON(http.StatusOK, response(fmt.Sprintf("calculation bulk %s has been deleted", calcBulkName), http.StatusOK))
	}

}

func (s *server) deleteWorkerPool(c *gin.Context) {
	workerPoolName := c.Param("id")
	workerpool := &workersv1.WorkerPool{}
	err := s.client.Get(s.ctx, ctrlruntimeclient.ObjectKey{Namespace: s.namespace, Name: workerPoolName}, workerpool)
	if err != nil {
		responseError(c, fmt.Sprintf("failed to get the workerpool %s", workerPoolName), err)
	}

	if err := s.client.Delete(s.ctx, workerpool); err != nil {
		responseError(c, fmt.Sprintf("failed to delete the workerpool %s", workerPoolName), err)
	} else {
		c.JSON(http.StatusOK, response(fmt.Sprintf("workerpool %s has been deleted", workerPoolName), http.StatusOK))
	}

}

func (s *server) deleteCalculation(c *gin.Context) {
	calcID := c.Param("id")
	calc := &v1.Calculation{}
	err := s.client.Get(s.ctx, ctrlruntimeclient.ObjectKey{Namespace: s.namespace, Name: calcID}, calc)
	if err != nil {
		responseError(c, fmt.Sprintf("failed to get calculation %s", calcID), err)
	}

	if err := s.client.Delete(s.ctx, calc); err != nil {
		responseError(c, "couldn't delete calculation", err)
	} else {
		c.JSON(http.StatusOK, response(fmt.Sprintf("calculation %q has been deleted", calcID), http.StatusOK))
	}
}

func (s *server) getCalculationBulks(c *gin.Context) {
	s.logger.WithFields(logrus.Fields{"host": c.Request.Host, "url": c.Request.URL, "method": c.Request.Method, "user-agent": c.Request.UserAgent()}).Info("getting calculations")

	var bulkList bulkv1.CalculationBulkList
	if err := s.client.List(s.ctx, &bulkList); err != nil {
		responseError(c, "couldn't get calculations list", err)
	} else {
		c.JSON(http.StatusOK, gin.H{"data": bulkList})
	}
}

func (s *server) getCalculationBulkByName(c *gin.Context) {
	bulkID := c.Param("id")

	bulk := &bulkv1.CalculationBulk{}
	err := s.client.Get(s.ctx, ctrlruntimeclient.ObjectKey{Namespace: s.namespace, Name: bulkID}, bulk)
	if err != nil {
		responseError(c, fmt.Sprintf("failed to get calculation bulk %s", bulkID), err)
	} else {
		c.JSON(http.StatusOK, gin.H{"data": bulk})
	}
}

func (s *server) getCalculations(c *gin.Context) {
	s.logger.WithFields(logrus.Fields{"host": c.Request.Host, "url": c.Request.URL, "method": c.Request.Method, "user-agent": c.Request.UserAgent()}).Info("getting calculations")

	var calcList v1.CalculationList
	if err := s.client.List(s.ctx, &calcList); err != nil {
		responseError(c, "couldn't get calculations list", err)
	} else {
		c.JSON(http.StatusOK, gin.H{"data": calcList})
	}
}

func (s *server) getCalculationByName(c *gin.Context) {
	calcID := c.Param("id")

	calc := &v1.Calculation{}
	err := s.client.Get(s.ctx, ctrlruntimeclient.ObjectKey{Namespace: s.namespace, Name: calcID}, calc)
	if err != nil {
		responseError(c, fmt.Sprintf("failed to get calculation %s", calcID), err)
	} else {
		c.JSON(http.StatusOK, gin.H{"data": calc})
	}
}

func (s *server) getCalculation(c *gin.Context) {
	teff := c.Query("teff")
	logG := c.Query("logG")

	t, _ := strconv.ParseFloat(teff, 64)
	l, _ := strconv.ParseFloat(logG, 64)

	// TODO: implement a cache and list from there with MatchingFields
	var calcList v1.CalculationList
	if err := s.client.List(s.ctx, &calcList); err != nil {
		responseError(c, fmt.Sprintf("failed to get calculation with teff: %s and logG: %s", teff, logG), err)
	} else {
		var calcs []v1.Calculation
		for _, calc := range calcList.Items {
			if calc.Spec.Params.Teff == t && calc.Spec.Params.LogG == l {
				calcs = append(calcs, calc)
			}
		}
		c.JSON(http.StatusOK, gin.H{"data": calcs})
	}
}

func (s *server) getWorkerPools(c *gin.Context) {
	s.logger.WithFields(logrus.Fields{"host": c.Request.Host, "url": c.Request.URL, "method": c.Request.Method, "user-agent": c.Request.UserAgent()}).Info("getting workerpools")

	var workerPoolList workersv1.WorkerPoolList
	if err := s.client.List(s.ctx, &workerPoolList); err != nil {
		responseError(c, "couldn't get workerpool list", err)
	} else {
		c.JSON(http.StatusOK, gin.H{"data": workerPoolList})
	}
}

func (s *server) getWorkerPoolByName(c *gin.Context) {
	workerPoolName := c.Param("id")

	workerPool := &workersv1.WorkerPool{}
	err := s.client.Get(s.ctx, ctrlruntimeclient.ObjectKey{Namespace: s.namespace, Name: workerPoolName}, workerPool)
	if err != nil {
		responseError(c, fmt.Sprintf("failed to get the workerpool %s", workerPoolName), err)
	} else {
		c.JSON(http.StatusOK, gin.H{"data": workerPool})
	}
}

func response(message string, statusCode int) gin.H {
	return gin.H{
		"message":     message,
		"status_code": statusCode,
	}
}

func responseError(c *gin.Context, message string, err error) {
	statusCode := http.StatusBadRequest

	if err == nil {
		statusCode = http.StatusOK
	} else if kerrors.IsUnauthorized(err) {
		statusCode = http.StatusUnauthorized
	} else if kerrors.IsForbidden(err) {
		statusCode = http.StatusForbidden
	} else if kerrors.IsInternalError(err) {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("%s: %v", message, err), "status_code": statusCode})
}
