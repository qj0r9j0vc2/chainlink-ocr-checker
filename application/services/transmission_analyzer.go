package services

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"chainlink-ocr-checker/domain/entities"
	"chainlink-ocr-checker/domain/interfaces"
	"gopkg.in/yaml.v2"
)

// transmissionAnalyzer implements the TransmissionAnalyzer interface
type transmissionAnalyzer struct {
	logger interfaces.Logger
}

// NewTransmissionAnalyzer creates a new transmission analyzer
func NewTransmissionAnalyzer(logger interfaces.Logger) interfaces.TransmissionAnalyzer {
	return &transmissionAnalyzer{
		logger: logger,
	}
}

// AnalyzeObserverActivity analyzes observer participation
func (a *transmissionAnalyzer) AnalyzeObserverActivity(transmissions []entities.Transmission) ([]entities.ObserverActivity, error) {
	// Create a map to track observer activities
	observerMap := make(map[uint8]*entities.ObserverActivity)
	
	for _, tx := range transmissions {
		// Get or create observer activity
		activity, exists := observerMap[tx.ObserverIndex]
		if !exists {
			activity = &entities.ObserverActivity{
				ObserverIndex: tx.ObserverIndex,
				Address:       tx.TransmitterAddress,
				TotalCount:    0,
				DailyCount:    make(map[string]int),
				MonthlyCount:  make(map[string]int),
			}
			observerMap[tx.ObserverIndex] = activity
		}
		
		// Update counts
		activity.TotalCount++
		
		// Update daily count
		dayKey := tx.BlockTimestamp.Format("2006-01-02")
		activity.DailyCount[dayKey]++
		
		// Update monthly count
		monthKey := tx.BlockTimestamp.Format("2006-01")
		activity.MonthlyCount[monthKey]++
	}
	
	// Convert map to slice
	activities := make([]entities.ObserverActivity, 0, len(observerMap))
	for _, activity := range observerMap {
		activities = append(activities, *activity)
	}
	
	// Sort by observer index
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].ObserverIndex < activities[j].ObserverIndex
	})
	
	return activities, nil
}

// DetectAnomalies detects anomalies in transmission patterns
func (a *transmissionAnalyzer) DetectAnomalies(transmissions []entities.Transmission) ([]interfaces.TransmissionAnomaly, error) {
	anomalies := []interfaces.TransmissionAnomaly{}
	
	if len(transmissions) == 0 {
		return anomalies, nil
	}
	
	// Sort transmissions by round
	sort.Slice(transmissions, func(i, j int) bool {
		roundI := uint32(transmissions[i].Epoch)<<8 | uint32(transmissions[i].Round)
		roundJ := uint32(transmissions[j].Epoch)<<8 | uint32(transmissions[j].Round)
		return roundI < roundJ
	})
	
	// Check for missing rounds
	prevRound := uint32(transmissions[0].Epoch)<<8 | uint32(transmissions[0].Round)
	for i := 1; i < len(transmissions); i++ {
		currRound := uint32(transmissions[i].Epoch)<<8 | uint32(transmissions[i].Round)
		
		if currRound > prevRound+1 {
			anomaly := interfaces.TransmissionAnomaly{
				Type:        interfaces.AnomalyTypeMissingRound,
				Description: fmt.Sprintf("Missing rounds between %d and %d", prevRound, currRound),
				Severity:    interfaces.AnomalySeverityMedium,
				Timestamp:   transmissions[i].BlockTimestamp.Unix(),
				Details: map[string]interface{}{
					"start_round": prevRound,
					"end_round":   currRound,
					"gap":         currRound - prevRound - 1,
				},
			}
			anomalies = append(anomalies, anomaly)
		}
		
		prevRound = currRound
	}
	
	// Check for duplicate rounds
	roundMap := make(map[uint32][]entities.Transmission)
	for _, tx := range transmissions {
		round := uint32(tx.Epoch)<<8 | uint32(tx.Round)
		roundMap[round] = append(roundMap[round], tx)
	}
	
	for round, txs := range roundMap {
		if len(txs) > 1 {
			anomaly := interfaces.TransmissionAnomaly{
				Type:        interfaces.AnomalyTypeDuplicateRound,
				Description: fmt.Sprintf("Duplicate transmissions for round %d", round),
				Severity:    interfaces.AnomalySeverityHigh,
				Timestamp:   txs[0].BlockTimestamp.Unix(),
				Details: map[string]interface{}{
					"round":       round,
					"count":       len(txs),
					"transmitters": func() []string {
						addrs := make([]string, len(txs))
						for i, tx := range txs {
							addrs[i] = tx.TransmitterAddress.Hex()
						}
						return addrs
					}(),
				},
			}
			anomalies = append(anomalies, anomaly)
		}
	}
	
	// Check for inactive observers
	observerActivity := make(map[uint8]int)
	for _, tx := range transmissions {
		observerActivity[tx.ObserverIndex]++
	}
	
	// Assume we should have activity from all observers 0-30
	expectedObservers := 31
	for i := uint8(0); i < uint8(expectedObservers); i++ {
		if count, exists := observerActivity[i]; !exists || count == 0 {
			anomaly := interfaces.TransmissionAnomaly{
				Type:        interfaces.AnomalyTypeInactiveObserver,
				Description: fmt.Sprintf("Observer %d has no transmissions", i),
				Severity:    interfaces.AnomalySeverityLow,
				Timestamp:   time.Now().Unix(),
				Details: map[string]interface{}{
					"observer_index": i,
				},
			}
			anomalies = append(anomalies, anomaly)
		}
	}
	
	// Check for high latency
	for i := 1; i < len(transmissions); i++ {
		timeDiff := transmissions[i].BlockTimestamp.Sub(transmissions[i-1].BlockTimestamp)
		if timeDiff > 5*time.Minute { // Assuming 5 minutes is too long between rounds
			anomaly := interfaces.TransmissionAnomaly{
				Type:        interfaces.AnomalyTypeHighLatency,
				Description: fmt.Sprintf("High latency of %s between rounds", timeDiff),
				Severity:    interfaces.AnomalySeverityMedium,
				Timestamp:   transmissions[i].BlockTimestamp.Unix(),
				Details: map[string]interface{}{
					"latency_seconds": timeDiff.Seconds(),
					"from_round":      uint32(transmissions[i-1].Epoch)<<8 | uint32(transmissions[i-1].Round),
					"to_round":        uint32(transmissions[i].Epoch)<<8 | uint32(transmissions[i].Round),
				},
			}
			anomalies = append(anomalies, anomaly)
		}
	}
	
	return anomalies, nil
}

// GenerateReport generates a comprehensive report
func (a *transmissionAnalyzer) GenerateReport(transmissions []entities.Transmission, format interfaces.OutputFormat) ([]byte, error) {
	// Analyze observer activity
	activities, err := a.AnalyzeObserverActivity(transmissions)
	if err != nil {
		return nil, err
	}
	
	// Detect anomalies
	anomalies, err := a.DetectAnomalies(transmissions)
	if err != nil {
		return nil, err
	}
	
	// Create report structure
	report := map[string]interface{}{
		"summary": map[string]interface{}{
			"total_transmissions": len(transmissions),
			"total_observers":     len(activities),
			"total_anomalies":     len(anomalies),
			"date_range": map[string]interface{}{
				"start": func() string {
					if len(transmissions) > 0 {
						return transmissions[0].BlockTimestamp.Format(time.RFC3339)
					}
					return ""
				}(),
				"end": func() string {
					if len(transmissions) > 0 {
						return transmissions[len(transmissions)-1].BlockTimestamp.Format(time.RFC3339)
					}
					return ""
				}(),
			},
		},
		"observer_activities": activities,
		"anomalies":          anomalies,
	}
	
	// Generate output based on format
	switch format {
	case interfaces.OutputFormatJSON:
		return json.MarshalIndent(report, "", "  ")
	case interfaces.OutputFormatYAML:
		return yaml.Marshal(report)
	default:
		return json.MarshalIndent(report, "", "  ")
	}
}