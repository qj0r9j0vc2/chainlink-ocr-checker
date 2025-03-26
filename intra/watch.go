package intra

import (
	"context"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	ocr2aggregator "github.com/smartcontractkit/libocr/gethwrappers2/accesscontrolledocr2aggregator"
)

func FetchLatestN(client *ethclient.Client, contractAddr common.Address, queryRoundNum, queryWindow int) (*QueryResult, error) {
	aggr, err := ocr2aggregator.NewAccessControlledOCR2Aggregator(contractAddr, client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OCR2 aggregator instance")
	}

	desc, err := aggr.Description(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get description")
	}

	log.Debugf("%s: %s", contractAddr, desc)

	latestRoundData, err := aggr.LatestRoundData(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latestRoundData")
	}

	latestBlock, err := client.BlockNumber(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest block number")
	}

	var roundIds []uint32
	for i := latestRoundData.RoundId.Uint64(); i > latestRoundData.RoundId.Uint64()-uint64(queryRoundNum); i-- {
		roundIds = append(roundIds, uint32(i))
	}

	startBlock := big.NewInt(int64(latestBlock - uint64(queryWindow)))
	endBlock := big.NewInt(int64(latestBlock))

	transmittersMap := make(map[[32]byte][]common.Address)
	if latestCfgDetail, err := aggr.LatestConfigDetails(nil); err == nil {
		if txs, err := aggr.GetTransmitters(nil); err == nil {
			transmittersMap[latestCfgDetail.ConfigDigest] = txs
		}
	}

	sem := make(chan struct{}, maxConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for from := new(big.Int).Set(startBlock); from.Cmp(endBlock) <= 0; {
		to := new(big.Int).Add(from, big.NewInt(int64(queryWindow-1)))
		if to.Cmp(endBlock) > 0 {
			to.Set(endBlock)
		}

		start := from.Uint64()
		end := to.Uint64()

		sem <- struct{}{}
		wg.Add(1)
		go func(start, end uint64) {
			defer wg.Done()
			defer func() { <-sem }()

			iter, err := aggr.FilterConfigSet(&bind.FilterOpts{
				Start: start,
				End:   &end,
			})
			if err != nil {
				log.Warnf("failed to filter config for round %d: %v", start, err)
				return
			}
			for iter.Next() {
				mu.Lock()
				transmittersMap[iter.Event.ConfigDigest] = iter.Event.Transmitters
				mu.Unlock()
			}
			_ = iter.Close()
		}(start, end)

		from.Add(to, big.NewInt(1))
	}
	wg.Wait()

	output, err := filterAndCaptureTransmissions(aggr, startBlock.Uint64(), endBlock.Uint64(), roundIds, transmittersMap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to capture transmissions")
	}

	return &QueryResult{Output: output}, nil
}
