package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	proto "github.com/vega-project/ccb-operator/proto"

	"gorm.io/gorm"
)

type CalculationResultsStore interface {
	StoreOrUpdateData(ctx context.Context, in *proto.StoreRequest) (*proto.StoreResponse, error)
	GetData(ctx context.Context, parameters map[string]string) (*CalculationResults, error)
}

type calculationResultsStore struct {
	db *gorm.DB
}

func (s *calculationResultsStore) StoreOrUpdateData(ctx context.Context, in *proto.StoreRequest) (*proto.StoreResponse, error) {
	parametersJson, err := json.Marshal(in.Parameters)
	if err != nil {
		return nil, err
	}

	existingData, err := s.GetData(ctx, in.Parameters)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		existingData.Results = in.Results
		if err := s.db.Save(existingData).Error; err != nil {
			return nil, err
		}
		return &proto.StoreResponse{Message: "Data updated successfully"}, nil
	}

	if err := s.db.Create(&CalculationResults{ParametersJSON: string(parametersJson), Results: in.Results}).Error; err != nil {
		return nil, err
	}

	return &proto.StoreResponse{Message: "Data stored successfully"}, nil
}

func (s *calculationResultsStore) GetData(ctx context.Context, parameters map[string]string) (*CalculationResults, error) {
	query := s.db.Model(&CalculationResults{})

	for key, value := range parameters {
		jsonQuery := fmt.Sprintf("parameters_json ->> '%s' = ?", key)
		query = query.Where(jsonQuery, value)
	}

	var result CalculationResults
	if err := query.First(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to find CalculationResults: %w", err)
	}

	return &result, nil
}
