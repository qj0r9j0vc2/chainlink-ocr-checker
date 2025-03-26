package intra

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	ocr2aggregator "github.com/smartcontractkit/libocr/gethwrappers2/accesscontrolledocr2aggregator"
	"math/big"
)

func FetchLatestN(client *ethclient.Client, contractAddr common.Address, lastRoundNum, lastCheckBlock, querySize uint64, resultChan chan QueryResult) error {
	aggr, err := ocr2aggregator.NewAccessControlledOCR2Aggregator(contractAddr, client)
	if err != nil {
		return errors.Wrap(err, "failed to create OCR2 aggregator instance")
	}

	latestRoundData, err := aggr.LatestRoundData(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get latestRoundData")
	}

	block, err := client.BlockNumber(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to get block number")
	}

	startRound := latestRoundData.RoundId.Uint64() - uint64(lastRoundNum)
	endRound := latestRoundData.RoundId.Uint64()

	startBlock := big.NewInt(int64(block - lastCheckBlock))
	endBlock := big.NewInt(int64(block))

	log.Debugf("%s: fetching events from block %d to %d", contractAddr.Hex(), startBlock, endBlock)

	return fetch(
		aggr,
		startBlock,
		endBlock,
		int64(startRound),
		int64(endRound),
		int64(querySize),
		resultChan,
	)
}
