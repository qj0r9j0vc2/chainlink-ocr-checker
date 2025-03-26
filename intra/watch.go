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

func FetchLatestN(client *ethclient.Client, contractAddr common.Address, queryRoundNum, queryWindow int, resultChan chan QueryResult) error {
	aggr, err := ocr2aggregator.NewAccessControlledOCR2Aggregator(contractAddr, client)
	if err != nil {
		return errors.Wrap(err, "failed to create OCR2 aggregator instance")
	}

	desc, err := aggr.Description(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get description")
	}

	latestRoundData, err := aggr.LatestRoundData(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get latestRoundData")
	}

	latestBlock, err := client.BlockNumber(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to get latest block number")
	}

	var roundIds []uint32
	if latestRoundData.RoundId.Uint64() <= uint64(queryRoundNum) {
		queryRoundNum = int(latestRoundData.RoundId.Uint64())
	}
	for i := int(latestRoundData.RoundId.Uint64()); i > int(latestRoundData.RoundId.Uint64())-queryRoundNum; i-- {
		roundIds = append(roundIds, uint32(i))
	}

	startBlock := big.NewInt(int64(latestBlock - uint64(queryWindow)))
	endBlock := big.NewInt(int64(latestBlock))

	startBlock, err = getBlockNumberByRoundId(client, aggr, int64(roundIds[queryRoundNum-1]))
	if err != nil {
		return errors.Wrapf(err, "failed to get latest block number")
	}

	transmittersMap := make(map[[32]byte][]common.Address)
	if latestCfgDetail, err := aggr.LatestConfigDetails(nil); err == nil {
		if txs, err := aggr.GetTransmitters(nil); err == nil {
			transmittersMap[latestCfgDetail.ConfigDigest] = txs
		}
	}
	log.Debugf("%s (%s) : latest round data: %s(%v), start: %d, end: %d",
		contractAddr,
		desc,
		latestRoundData.RoundId,
		latestRoundData.StartedAt,
		startBlock,
		endBlock)

	return Fetch(client, contractAddr, int64(roundIds[len(roundIds)-1]), int64(roundIds[0]), int64(queryWindow), resultChan)
}
