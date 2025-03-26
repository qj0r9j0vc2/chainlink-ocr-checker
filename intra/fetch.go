package intra

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
	maxConcurrency       = 16 // Limit for concurrent RPC calls
)

type QueryResult struct {
	StartBlock uint64
	Output     []config.Result
	Err        error
}

func Fetch(client *ethclient.Client, contractAddr common.Address, startRound, endRound, querySize int64, resultChan chan QueryResult) error {
	aggr, err := ocr2aggregator.NewAccessControlledOCR2Aggregator(contractAddr, client)
	if err != nil {
		return errors.Wrap(err, "failed to create OCR2 aggregator instance")
	}

	desc, err := aggr.Description(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get description")
	}
	log.Infof("%s: %s", contractAddr, desc)

	var (
		startBlock *big.Int
		endBlock   *big.Int
		errs       = make(chan error, 2)
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		block, err := getBlockNumberByRoundId(client, aggr, startRound)
		if err != nil {
			errs <- errors.Wrapf(err, "getting start block for round %d", startRound)
			return
		}
		startBlock = block
	}()

	go func() {
		defer wg.Done()
		block, err := getBlockNumberByRoundId(client, aggr, endRound)
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

	if endBlock.Cmp(startBlock) < 0 {
		return errors.New("invalid block range: startBlock > endBlock")
	}

	log.Printf("Fetching events from block %d to %d", startBlock, endBlock)

	var roundIds []uint32
	for i := startRound; i <= endRound; i++ {
		roundIds = append(roundIds, uint32(i))
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

			output, err := filterAndCaptureTransmissions(aggr, start, end, roundIds, transmittersMap)
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

func getBlockNumberByRoundId(client *ethclient.Client, aggr *ocr2aggregator.AccessControlledOCR2Aggregator, roundId int64) (*big.Int, error) {
	ts, err := aggr.GetTimestamp(nil, big.NewInt(roundId))
	if err != nil {
		return nil, fmt.Errorf("GetTimestamp failed for round %d: %w", roundId, err)
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
	roundIds []uint32,
	transmittersMap map[[32]byte][]common.Address,
) ([]config.Result, error) {
	opts := &bind.FilterOpts{Start: start, End: &end, Context: context.Background()}
	iter, err := aggr.FilterNewTransmission(opts, roundIds)
	if err != nil {
		return nil, fmt.Errorf("filtering transmissions failed: %w", err)
	}
	defer iter.Close()

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
			RoundId:      fmt.Sprintf("%d", iter.Event.AggregatorRoundId),
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
	latestBlock, err := client.BlockByNumber(ctx, big.NewInt(int64(latestBlockNumber)))
	if err != nil {
		return nil, nil, err
	}
	latestTimestamp := big.NewInt(int64(latestBlock.Time()))

	blockInterval := defaultBlockInterval
	if block, err := client.BlockByNumber(ctx, big.NewInt(int64(latestBlockNumber))); err == nil {
		if prev, err := client.BlockByNumber(ctx, big.NewInt(int64(latestBlockNumber-1))); err == nil && block.Time() > prev.Time() {
			blockInterval = int(block.Time() - prev.Time())
		}
	}

	diffSeconds := new(big.Int).Sub(latestTimestamp, targetTimestamp).Int64()
	estimatedBlocksAgo := diffSeconds / int64(blockInterval)
	if estimatedBlocksAgo < 0 {
		estimatedBlocksAgo = 0
	}
	estimatedStart := new(big.Int).Sub(big.NewInt(int64(latestBlockNumber)), big.NewInt(estimatedBlocksAgo))
	if estimatedStart.Sign() < 0 {
		estimatedStart = big.NewInt(0)
	}

	low := estimatedStart
	high := big.NewInt(int64(latestBlockNumber))
	mid := new(big.Int)
	for low.Cmp(high) <= 0 {
		mid.Add(low, high)
		mid.Div(mid, big.NewInt(2))
		block, err := client.BlockByNumber(ctx, mid)
		if err != nil {
			return nil, nil, err
		}
		blockTime := big.NewInt(int64(block.Time()))
		cmp := blockTime.Cmp(targetTimestamp)
		if cmp < 0 {
			low.Add(mid, big.NewInt(1))
		} else if cmp > 0 {
			high.Sub(mid, big.NewInt(1))
		} else {
			return mid, block, nil
		}
	}
	closestBlock, err := client.BlockByNumber(ctx, low)
	if err != nil {
		return nil, nil, err
	}
	return low, closestBlock, nil
}
