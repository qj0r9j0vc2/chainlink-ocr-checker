package usecases

import (
	"context"
	"testing"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/errors"
	"chainlink-ocr-checker/domain/interfaces"
	"chainlink-ocr-checker/test/helpers"
	"chainlink-ocr-checker/test/mocks"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchTransmissionsUseCase_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	
	mockFetcher := mocks.NewMockTransmissionFetcher(ctrl)
	mockRepo := mocks.NewMockTransmissionRepository(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)
	
	// Set up logger expectations
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
	
	useCase := NewFetchTransmissionsUseCase(mockFetcher, mockRepo, mockLogger)
	ctx := context.Background()
	
	t.Run("successful fetch", func(t *testing.T) {
		contractAddr := helpers.RandomAddress()
		params := interfaces.FetchTransmissionsParams{
			ContractAddress: contractAddr,
			StartRound:      1,
			EndRound:        10,
		}
		
		expectedResult := &entities.TransmissionResult{
			ContractAddress: contractAddr,
			StartRound:      1,
			EndRound:        10,
			Transmissions: []entities.Transmission{
				{
					ContractAddress:    contractAddr,
					Epoch:              0,
					Round:              1,
					TransmitterAddress: helpers.RandomAddress(),
					BlockNumber:        12345,
					BlockTimestamp:     time.Now(),
				},
			},
		}
		
		mockFetcher.EXPECT().
			FetchByRounds(ctx, contractAddr, uint32(1), uint32(10)).
			Return(expectedResult, nil)
		
		mockRepo.EXPECT().
			SaveBatch(ctx, expectedResult.Transmissions).
			Return(nil)
		
		result, err := useCase.Execute(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})
	
	t.Run("validation error - invalid contract", func(t *testing.T) {
		params := interfaces.FetchTransmissionsParams{
			ContractAddress: common.Address{},
			StartRound:      1,
			EndRound:        10,
		}
		
		result, err := useCase.Execute(ctx, params)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "validation failed for 1 fields")
		validErr, ok := err.(*errors.ValidationError)
		require.True(t, ok)
		assert.Contains(t, validErr.Fields["contract_address"][0], "contract address is required")
	})
	
	t.Run("validation error - invalid round range", func(t *testing.T) {
		params := interfaces.FetchTransmissionsParams{
			ContractAddress: helpers.RandomAddress(),
			StartRound:      10,
			EndRound:        1,
		}
		
		result, err := useCase.Execute(ctx, params)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "validation failed for 1 fields")
		validErr, ok := err.(*errors.ValidationError)
		require.True(t, ok)
		assert.Contains(t, validErr.Fields["rounds"][0], "invalid range")
	})
	
	t.Run("fetch error", func(t *testing.T) {
		contractAddr := helpers.RandomAddress()
		params := interfaces.FetchTransmissionsParams{
			ContractAddress: contractAddr,
			StartRound:      1,
			EndRound:        10,
		}
		
		mockFetcher.EXPECT().
			FetchByRounds(ctx, contractAddr, uint32(1), uint32(10)).
			Return(nil, assert.AnError)
		
		result, err := useCase.Execute(ctx, params)
		require.Error(t, err)
		assert.Nil(t, result)
	})
	
	t.Run("save error - continues without failing", func(t *testing.T) {
		contractAddr := helpers.RandomAddress()
		params := interfaces.FetchTransmissionsParams{
			ContractAddress: contractAddr,
			StartRound:      1,
			EndRound:        10,
		}
		
		expectedResult := &entities.TransmissionResult{
			ContractAddress: contractAddr,
			StartRound:      1,
			EndRound:        10,
			Transmissions: []entities.Transmission{
				{
					ContractAddress: contractAddr,
					Epoch:           0,
					Round:           1,
				},
			},
		}
		
		mockFetcher.EXPECT().
			FetchByRounds(ctx, contractAddr, uint32(1), uint32(10)).
			Return(expectedResult, nil)
		
		mockRepo.EXPECT().
			SaveBatch(ctx, expectedResult.Transmissions).
			Return(assert.AnError)
		
		// Should still return result even if save fails
		result, err := useCase.Execute(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})
}