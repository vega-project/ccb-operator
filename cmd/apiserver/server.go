package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	v1 "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/typed/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type server struct {
	logger       *logrus.Entry
	ctx          context.Context
	clientGetter func(token string) v1.VegaV1Interface
}

func (s *server) createCalculation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	decoder := json.NewDecoder(r.Body)

	var calc struct {
		Teff string `json:"teff"`
		LogG string `json:"logG"`
	}
	err := decoder.Decode(&calc)
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("couldn't decode json params: %v", err), determineStatusCodeByError(err)))
		return
	}

	t, err := strconv.ParseFloat(calc.Teff, 64)
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("teff is not a valid float64 number: %v", err), determineStatusCodeByError(err)))
		return
	}

	l, err := strconv.ParseFloat(calc.LogG, 64)
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("logG is not a valid float64 number: %v", err), determineStatusCodeByError(err)))
		return
	}

	calculation := util.NewCalculation(t, l)
	calculation.Labels = map[string]string{"created_by_human": "true"}

	c, err := s.clientGetter(r.Header.Get("X-Session-Token")).Calculations().Create(s.ctx, calculation, metav1.CreateOptions{})
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("couldn't create calculation: %v", err), determineStatusCodeByError(err)))
	} else {
		json.NewEncoder(w).Encode(c)
	}

}

func (s *server) deleteCalculation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		return
	}

	calcID := mux.Vars(r)["id"]
	err := s.clientGetter(r.Header.Get("X-Session-Token")).Calculations().Delete(s.ctx, calcID, metav1.DeleteOptions{})
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("couldn't delete calculation: %v", err), determineStatusCodeByError(err)))
	} else {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("calculation %q has been deleted", calcID), http.StatusOK))
	}
}

func (s *server) getCalculations(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	s.logger.WithFields(logrus.Fields{"host": r.Host, "url": r.URL, "method": r.Method, "user-agent": r.UserAgent()}).Info("getting calculations")

	calcList, err := s.clientGetter(r.Header.Get("X-Session-Token")).Calculations().List(s.ctx, metav1.ListOptions{})
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("couldn't get calculations list: %v", err), determineStatusCodeByError(err)))
	} else {
		json.NewEncoder(w).Encode(calcList)
	}
}

func (s *server) getCalculationByName(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	calcID := mux.Vars(r)["id"]
	calc, err := s.clientGetter(r.Header.Get("X-Session-Token")).Calculations().Get(s.ctx, calcID, metav1.GetOptions{})
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("couldn't get calculation %s: %v", calcID, err), determineStatusCodeByError(err)))
	} else {
		json.NewEncoder(w).Encode(calc)
	}
}

func (s *server) getCalculation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	r.ParseForm()

	teff := r.Form.Get("teff")
	logG := r.Form.Get("logG")

	t, err := strconv.ParseFloat(teff, 64)
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("teff is not a valid float64 number: %v", err), determineStatusCodeByError(err)))
		return
	}

	l, err := strconv.ParseFloat(logG, 64)
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("logG is not a valid float64 number: %v", err), determineStatusCodeByError(err)))
		return
	}

	calcName := util.GetCalculationName(t, l)
	calc, err := s.clientGetter(r.Header.Get("X-Session-Token")).Calculations().Get(s.ctx, calcName, metav1.GetOptions{})
	if err != nil {
		json.NewEncoder(w).Encode(response(fmt.Sprintf("couldn't get calculation %s: %v", calcName, err), determineStatusCodeByError(err)))
	} else {
		json.NewEncoder(w).Encode(calc)
	}
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

func determineStatusCodeByError(err error) int {
	if err == nil {
		return http.StatusOK
	}

	if kerrors.IsUnauthorized(err) {
		return http.StatusUnauthorized
	}

	if kerrors.IsForbidden(err) {
		return http.StatusForbidden
	}

	if kerrors.IsInternalError(err) {
		return http.StatusInternalServerError
	}

	return http.StatusBadRequest
}
