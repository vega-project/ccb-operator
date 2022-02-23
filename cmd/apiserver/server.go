package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	kerrors "k8s.io/apimachinery/pkg/api/errors"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type server struct {
	logger      *logrus.Entry
	namespace   string
	ctx         context.Context
	client      ctrlruntimeclient.Client
	resultsPath string
}

func (s *server) createCalculation(c *gin.Context) {
	decoder := json.NewDecoder(c.Request.Body)

	var calc struct {
		Teff string `json:"teff"`
		LogG string `json:"logG"`
	}
	err := decoder.Decode(&calc)
	if err != nil {
		responseError(c, "couldn't decode json params", err)
		return
	}

	t, err := strconv.ParseFloat(calc.Teff, 64)
	if err != nil {
		responseError(c, "couldn't parse teff as a float number", err)
		return
	}

	l, err := strconv.ParseFloat(calc.LogG, 64)
	if err != nil {
		responseError(c, "couldn't parse logG as a float number", err)
		return
	}

	s.logger.WithField("teff", t).WithField("logG", l).Info("Creating calculation...")
	calculation := util.NewCalculation(t, l)
	calculation.Labels = map[string]string{"created_by_human": "true"}
	calculation.Namespace = s.namespace
	if err := s.client.Create(s.ctx, calculation); err != nil {
		responseError(c, "couldn't create calculation", err)
	} else {
		c.JSON(http.StatusOK, gin.H{"data": calculation})
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
			if calc.Spec.Teff == t && calc.Spec.LogG == l {
				calcs = append(calcs, calc)
			}
		}
		c.JSON(http.StatusOK, gin.H{"data": calcs})
	}
}

func (s *server) sendResults(c *gin.Context, teff, logG float64) {
	resultDirName := fmt.Sprintf("%.1f___%.2f", teff, logG)

	fort7Data, err := ioutil.ReadFile(filepath.Join(s.resultsPath, resultDirName, "fort.7"))
	if err != nil {
		responseError(c, "File reading error", err)
		return
	}

	fort8Data, err := ioutil.ReadFile(filepath.Join(s.resultsPath, resultDirName, "fort.8"))
	if err != nil {
		responseError(c, "File reading error", err)
		return
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var files = []struct {
		name string
		data []byte
	}{
		{"fort.7", fort7Data},
		{"fort.8", fort8Data},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.name,
			Mode: 0600,
			Size: int64(len(file.data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			responseError(c, "couldn't write header while creating the tar file", err)
			return
		}
		if _, err := tw.Write(file.data); err != nil {
			responseError(c, "couldn't write data while creating the tar file", err)
			return
		}
	}

	if err := tw.Close(); err != nil {
		responseError(c, "couldn't close the tar file", err)
		return
	}

	tarBytes := buf.Bytes()
	c.Writer.Header().Add("Content-Disposition", "attachment; filename="+fmt.Sprintf("%.1f_%.2f-results.tar.gz", teff, logG))
	c.Writer.Header().Add("Content-Type", http.DetectContentType(tarBytes))

	tarReader := bytes.NewReader(tarBytes)
	if _, err := io.Copy(c.Writer, tarReader); err != nil {
		responseError(c, "couldn't copy data into response writer", err)
	}
}

func (s *server) getCalculationResultsByID(c *gin.Context) {
	calcID := c.Param("id")

	calc := &v1.Calculation{}
	err := s.client.Get(s.ctx, ctrlruntimeclient.ObjectKey{Namespace: s.namespace, Name: calcID}, calc)
	if err != nil {
		responseError(c, fmt.Sprintf("couldn't get calculation %s", calcID), err)
		return
	}

	s.sendResults(c, calc.Spec.Teff, calc.Spec.LogG)
}

func (s *server) getCalculationResults(c *gin.Context) {
	teff := c.Query("teff")
	logG := c.Query("logg")

	t, err := strconv.ParseFloat(teff, 64)
	if err != nil {
		responseError(c, "couldn't parse teff as a float number", err)
		return
	}

	l, err := strconv.ParseFloat(logG, 64)
	if err != nil {
		responseError(c, "couldn't parse logG as a float number", err)
		return
	}

	s.sendResults(c, t, l)
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
