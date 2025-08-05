// Package internal contains internal implementation details for the OCR checker.
// It provides utilities for fetching and processing blockchain data.
package internal

import (
	"chainlink-ocr-checker/config"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	ocr2aggregator "github.com/smartcontractkit/libocr/gethwrappers2/accesscontrolledocr2aggregator"
	"math/big"
	"sync"
	"time"
)

const (
	defaultBlockInterval = 10
	maxConcurrency       = 30 // Limit for concurrent RPC calls
)

// QueryResult represents the result of a query operation.
type QueryResult struct {
	StartBlock uint64
	Output     []config.Result
	Err        error
}

// FetchPeriod fetches OCR transmissions for a given period range.
func FetchPeriod(
	client *ethclient.Client,
	contractAddr common.Address,
	startRound, endRound, querySize int64,
	resultChan chan QueryResult,
) error {
	aggr, err := ocr2aggregator.NewAccessControlledOCR2Aggregator(contractAddr, client)
	if err != nil {
		return errors.Wrap(err, "failed to create OCR2 aggregator instance")
	}

	desc, err := aggr.Description(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get description")
	}
	log.Debugf("%s: %s", contractAddr, desc)

	var (
		startBlock *big.Int
		endBlock   *big.Int
		errs       = make(chan error, 2)
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		block, err := getBlockNumberByRoundID(client, aggr, startRound)
		if err != nil {
			errs <- errors.Wrapf(err, "getting start block for round %d", startRound)
			return
		}
		startBlock = block
	}()

	go func() {
		defer wg.Done()
		block, err := getBlockNumberByRoundID(client, aggr, endRound)
		if err != nil {
			errs <- errors.Wrapf(err, "getting end block for round %d", endRound)
			return
		}
		endBlock = block
	}()

	wg.Wait()
	close(errs)
	if err = <-errs; err != nil {
		return err
	}
	log.Debugf("%s: fetching events from block %d to %d", contractAddr.Hex(), startBlock, endBlock)

	return fetch(
		aggr,
		startBlock,
		endBlock,
		startRound,
		endRound,
		querySize,
		resultChan,
	)
}

func fetch(
	aggr *ocr2aggregator.AccessControlledOCR2Aggregator,
	startBlock, endBlock *big.Int,
	startRound, endRound, querySize int64,
	resultChan chan QueryResult,
) error {
	if endBlock.Cmp(startBlock) < 0 {
		return errors.New("invalid block range: startBlock > endBlock")
	}

	var roundIDs []uint32
	for i := startRound; i <= endRound; i++ {
		roundIDs = append(roundIDs, uint32(i)) // #nosec G115 -- i is bounded by startRound and endRound
	}

	transmittersMap := make(map[[32]byte][]common.Address)
	latestCfgDetail, err := aggr.LatestConfigDetails(nil)
	if err == nil {
		if transmitters, err := aggr.GetTransmitters(nil); err == nil {
			transmittersMap[latestCfgDetail.ConfigDigest] = transmitters
		}
	}

	// Throttle for config fetching
	cfgSem := make(chan struct{}, maxConcurrency)
	cfgWg := sync.WaitGroup{}
	for from := new(big.Int).Set(startBlock); from.Cmp(endBlock) <= 0; {
		to := new(big.Int).Add(from, big.NewInt(querySize-1))
		if to.Cmp(endBlock) > 0 {
			to.Set(endBlock)
		}
		start := from.Uint64()
		end := to.Uint64()

		cfgSem <- struct{}{}
		cfgWg.Add(1)
		go func(start, end uint64) {
			defer cfgWg.Done()
			defer func() { <-cfgSem }()

			iter, err := aggr.FilterConfigSet(&bind.FilterOpts{Start: start, End: &end})
			if err != nil {
				log.Warnf("failed to filter config (block %d-%d): %v", start, end, err)
				return
			}
			if iter.Error() != nil {
				log.Warnf("failed to filter config (block %d-%d): %v", start, end, iter.Error())
				return
			}

			for iter.Next() {
				transmittersMap[iter.Event.ConfigDigest] = iter.Event.Transmitters
				log.Infof("%x : %v", iter.Event.ConfigDigest, iter.Event.Transmitters)
			}
			_ = iter.Close()
		}(start, end)

		from.Add(to, big.NewInt(1))
	}
	cfgWg.Wait()

	// Transmission fetching
	querySem := make(chan struct{}, maxConcurrency)
	queryWg := sync.WaitGroup{}
	for from := new(big.Int).Set(startBlock); from.Cmp(endBlock) <= 0; {
		to := new(big.Int).Add(from, big.NewInt(querySize-1))
		if to.Cmp(endBlock) > 0 {
			to.Set(endBlock)
		}
		start := from.Uint64()
		end := to.Uint64()

		querySem <- struct{}{}
		queryWg.Add(1)
		go func(start, end uint64) {
			defer queryWg.Done()
			defer func() { <-querySem }()

			output, err := filterAndCaptureTransmissions(aggr, start, end, roundIDs, transmittersMap)
			resultChan <- QueryResult{StartBlock: start, Output: output, Err: err}
		}(start, end)

		from.Add(to, big.NewInt(1))
	}

	go func() {
		queryWg.Wait()
		close(resultChan)
	}()

	return nil
}

func getBlockNumberByRoundID(
	client *ethclient.Client,
	aggr *ocr2aggregator.AccessControlledOCR2Aggregator,
	roundID int64,
) (*big.Int, error) {
	ts, err := aggr.GetTimestamp(nil, big.NewInt(roundID))
	if err != nil {
		return nil, fmt.Errorf("GetTimestamp failed for round %d: %w", roundID, err)
	}
	blockNumber, _, err := findBlockByTimestamp(client, ts)
	if err != nil {
		return nil, fmt.Errorf("FindBlockByTimestamp failed: %w", err)
	}
	return blockNumber, nil
}

func filterAndCaptureTransmissions(
	aggr *ocr2aggregator.AccessControlledOCR2Aggregator,
	start, end uint64,
	roundIDs []uint32,
	transmittersMap map[[32]byte][]common.Address,
) ([]config.Result, error) {
	opts := &bind.FilterOpts{Start: start, End: &end, Context: context.Background()}
	iter, err := aggr.FilterNewTransmission(opts, roundIDs)
	if err != nil {
		return nil, fmt.Errorf("filtering transmissions failed: %w", err)
	}
	defer func() { _ = iter.Close() }()

	var output []config.Result
	for iter.Next() {
		transmitters := transmittersMap[iter.Event.ConfigDigest]

		var observers, formatted []config.ResultObserver
		for _, observer := range iter.Event.Observers {
			idx := int(rune(observer))
			if idx >= 0 && idx < len(transmitters) {
				observers = append(observers, config.ResultObserver{Idx: idx, Address: transmitters[idx]})
			}
		}
		for idx, addr := range transmitters {
			formatted = append(formatted, config.ResultObserver{Idx: idx, Address: addr})
		}
		output = append(output, config.Result{
			RoundID:      fmt.Sprintf("%d", iter.Event.AggregatorRoundId),
			Timestamp:    time.UnixMilli(int64(iter.Event.ObservationsTimestamp) * 1e3),
			Observers:    observers,
			Transmitters: formatted,
		})
	}
	return output, nil
}

func findBlockByTimestamp(client *ethclient.Client, targetTimestamp *big.Int) (*big.Int, *types.Block, error) {
	ctx := context.Background()
	latestBlockNumber, err := client.BlockNumber(ctx)
	if err != nil {
		return nil, nil, err
	}
	// #nosec G115 -- block number is valid
	latestBlock, err := client.BlockByNumber(ctx, big.NewInt(int64(latestBlockNumber)))
	if err != nil {
		return nil, nil, err
	}
	latestTimestamp := big.NewInt(int64(latestBlock.Time())) // #nosec G115 -- block timestamp is valid

	blockInterval := defaultBlockInterval
	// #nosec G115 -- block number is valid
	if block, err := client.BlockByNumber(ctx, big.NewInt(int64(latestBlockNumber))); err == nil {
		prevBlockNum := big.NewInt(int64(latestBlockNumber - 1)) // #nosec G115 -- block number is valid
		if prev, err := client.BlockByNumber(ctx, prevBlockNum); err == nil && block.Time() > prev.Time() {
			blockInterval = int(block.Time() - prev.Time()) // #nosec G115 -- block times are valid
		}
	}

	diffSeconds := new(big.Int).Sub(latestTimestamp, targetTimestamp).Int64()
	estimatedBlocksAgo := diffSeconds / int64(blockInterval)
	if estimatedBlocksAgo < 0 {
		estimatedBlocksAgo = 0
	}
	estimatedStart := new(big.Int).Sub(big.NewInt(int64(latestBlockNumber)), big.NewInt(estimatedBlocksAgo)) // #nosec G115
	if estimatedStart.Sign() < 0 {
		estimatedStart = big.NewInt(0)
	}

	low := estimatedStart
	high := big.NewInt(int64(latestBlockNumber)) // #nosec G115 -- block number is valid
	mid := new(big.Int)
	for low.Cmp(high) <= 0 {
		mid.Add(low, high)
		mid.Div(mid, big.NewInt(2))
		block, err := client.BlockByNumber(ctx, mid)
		if err != nil {
			return nil, nil, err
		}
		blockTime := big.NewInt(int64(block.Time())) // #nosec G115 -- block timestamp is valid
		cmp := blockTime.Cmp(targetTimestamp)
		switch {
		case cmp < 0:
			low.Add(mid, big.NewInt(1))
		case cmp > 0:
			high.Sub(mid, big.NewInt(1))
		default:
			return mid, block, nil
		}
	}
	closestBlock, err := client.BlockByNumber(ctx, low)
	if err != nil {
		return nil, nil, err
	}
	return low, closestBlock, nil
}
