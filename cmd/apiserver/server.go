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

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	v1 "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/typed/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type server struct {
	logger      *logrus.Entry
	ctx         context.Context
	client      v1.VegaV1Interface
	resultsPath string
}

func (s *server) createCalculation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	decoder := json.NewDecoder(r.Body)

	var calc struct {
		Teff string `json:"teff"`
		LogG string `json:"logG"`
	}
	err := decoder.Decode(&calc)
	if err != nil {
		responseError(w, "couldn't decode json params", err)
		return
	}

	t, err := strconv.ParseFloat(calc.Teff, 64)
	if err != nil {
		responseError(w, "couldn't parse teff as a float number", err)
		return
	}

	l, err := strconv.ParseFloat(calc.LogG, 64)
	if err != nil {
		responseError(w, "couldn't parse logG as a float number", err)
		return
	}

	calculation := util.NewCalculation(t, l)
	calculation.Labels = map[string]string{"created_by_human": "true"}

	c, err := s.client.Calculations().Create(s.ctx, calculation, metav1.CreateOptions{})
	if err != nil {
		responseError(w, "couldn't create calculation", err)
	} else {
		json.NewEncoder(w).Encode(c)
	}

}

func (s *server) deleteCalculation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	calcID := mux.Vars(r)["id"]
	err := s.client.Calculations().Delete(s.ctx, calcID, metav1.DeleteOptions{})
	if err != nil {
		responseError(w, "couldn't delete calculation", err)
	} else {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("calculation %q has been deleted", calcID), http.StatusOK))
	}
}

func (s *server) getCalculations(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	s.logger.WithFields(logrus.Fields{"host": r.Host, "url": r.URL, "method": r.Method, "user-agent": r.UserAgent()}).Info("getting calculations")

	calcList, err := s.client.Calculations().List(s.ctx, metav1.ListOptions{})
	if err != nil {
		responseError(w, "couldn't get calculations list", err)
	} else {
		json.NewEncoder(w).Encode(calcList)
	}
}

func (s *server) getCalculationByName(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	calcID := mux.Vars(r)["id"]
	calc, err := s.client.Calculations().Get(s.ctx, calcID, metav1.GetOptions{})
	if err != nil {
		responseError(w, fmt.Sprintf("couldn't get calculation %s", calcID), err)
	} else {
		json.NewEncoder(w).Encode(calc)
	}
}

func (s *server) getCalculation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	r.ParseForm()

	teff := r.Form.Get("teff")
	logG := r.Form.Get("logG")

	t, err := strconv.ParseFloat(teff, 64)
	if err != nil {
		responseError(w, "couldn't parse teff as a float number", err)
		return
	}

	l, err := strconv.ParseFloat(logG, 64)
	if err != nil {
		responseError(w, "couldn't parse logG as a float number", err)
		return
	}

	calcName := util.GetCalculationName(t, l)
	calc, err := s.client.Calculations().Get(s.ctx, calcName, metav1.GetOptions{})
	if err != nil {
		responseError(w, fmt.Sprintf("couldn't get calculation %s", calcName), err)
	} else {
		json.NewEncoder(w).Encode(calc)
	}
}

func (s *server) sendResults(w http.ResponseWriter, teff, logG float64) {
	resultDirName := fmt.Sprintf("%.1f___%.2f", teff, logG)

	fort7Data, err := ioutil.ReadFile(filepath.Join(s.resultsPath, resultDirName, "fort.7"))
	if err != nil {
		responseError(w, "File reading error", err)
		return
	}

	fort8Data, err := ioutil.ReadFile(filepath.Join(s.resultsPath, resultDirName, "fort.8"))
	if err != nil {
		responseError(w, "File reading error", err)
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
			responseError(w, "couldn't write header while creating the tar file", err)
			return
		}
		if _, err := tw.Write(file.data); err != nil {
			responseError(w, "couldn't write data while creating the tar file", err)
			return
		}
	}

	if err := tw.Close(); err != nil {
		responseError(w, "couldn't close the tar file", err)
		return
	}

	tarBytes := buf.Bytes()
	w.Header().Set("Content-Disposition", "attachment; filename="+fmt.Sprintf("%.1f_%.2f-results.tar.gz", teff, logG))
	w.Header().Set("Content-Type", http.DetectContentType(tarBytes))

	tarReader := bytes.NewReader(tarBytes)
	if _, err := io.Copy(w, tarReader); err != nil {
		responseError(w, "couldn't copy data into response writer", err)
	}
}

func (s *server) getCalculationResultsByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	calcID := mux.Vars(r)["id"]

	calc, err := s.client.Calculations().Get(s.ctx, calcID, metav1.GetOptions{})
	if err != nil {
		responseError(w, fmt.Sprintf("couldn't get calculation %s", calcID), err)
		return
	}
	s.sendResults(w, calc.Spec.Teff, calc.Spec.LogG)
}

func (s *server) getCalculationResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	teff := string(r.URL.Query().Get("teff"))
	logG := string(r.URL.Query().Get("logg"))

	t, err := strconv.ParseFloat(teff, 64)
	if err != nil {
		responseError(w, "couldn't parse teff as a float number", err)
		return
	}

	l, err := strconv.ParseFloat(logG, 64)
	if err != nil {
		responseError(w, "couldn't parse logG as a float number", err)
		return
	}

	s.sendResults(w, t, l)
}

type Response struct {
	Message    string `json:"message,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
}

func response(message string, statusCode int) Response {
	return Response{
		Message:    message,
		StatusCode: statusCode,
	}
}

func responseError(w http.ResponseWriter, message string, err error) {
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

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response(fmt.Sprintf("%s: %v", message, err), statusCode))
}
