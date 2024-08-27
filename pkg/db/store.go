package db

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	proto "github.com/vega-project/ccb-operator/proto"

	"gorm.io/gorm"
)

type CalculationResultsStore interface {
	StoreOrUpdateData(ctx context.Context, in *proto.StoreRequest) (*proto.StoreReply, error)
}

type calculationResultsStore struct {
	db *gorm.DB
}

func (s *calculationResultsStore) StoreOrUpdateData(ctx context.Context, in *proto.StoreRequest) (*proto.StoreReply, error) {
	var keys []string
	for k := range in.Parameters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sortedParameters []string
	for _, k := range keys {
		sortedParameters = append(sortedParameters, k, in.Parameters[k])
	}

	parametersJson, err := json.Marshal(sortedParameters)
	if err != nil {
		return nil, err
	}

	var existingData CalculationResults
	if err := s.db.Where("parameters_json = ?", parametersJson).First(&existingData).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		existingData.Results = in.Results
		if err := s.db.Save(&existingData).Error; err != nil {
			return nil, err
		}
		return &proto.StoreReply{Message: "Data updated successfully"}, nil
	}

	if err := s.db.Create(&CalculationResults{ParametersJSON: string(parametersJson), Results: in.Results}).Error; err != nil {
		return nil, err
	}

	return &proto.StoreReply{Message: "Data stored successfully"}, nil
}
