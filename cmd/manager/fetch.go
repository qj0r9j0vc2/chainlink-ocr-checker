package manager

import (
	"bufio"
	"chainlink-ocr-checker/config"
	"chainlink-ocr-checker/intra"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
	cli "github.com/spf13/cobra"
	"os"
	"strconv"
)

const (
	defaultBlockInterval = 13

	ETH_RPC      = "https://polygon.blockpi.network/v1/rpc/dabafa7e679ff4440d76100ca78a4435cb475f16"
	CONTRACT     = "0x039903dDc82D06ad885b7bc9d9D5A9e550b3416D"
	START_ROUND  = 129400
	END_ROUND    = 131400
	QUERY_WINDOW = 5000
)

var fetchCmd = &cli.Command{
	Use:     "fetch",
	Aliases: []string{"f"},
	Example: "ocr-checker fetch [contract] [start-round] [end-round]",
	Short:   "fetchCmd",
	Long:    `fetchCmd`,
	Args:    cli.ExactArgs(3),
	Run: func(cmd *cli.Command, args []string) {

		var (
			contract      = args[0]
			startRound, _ = strconv.ParseInt(args[1], 10, 64)
			endRound, _   = strconv.ParseInt(args[2], 10, 64)
		)

		contractAddr := common.HexToAddress(contract)

		log.Infof("contract: %s, start-round: %d, end-round: %d", contract, startRound, endRound)

		resultChan := make(chan intra.QueryResult)

		err := intra.Fetch(cfg.Network, contractAddr, int64(startRound), int64(endRound), QUERY_WINDOW, resultChan)
		if err != nil {
			cfg.Error(err)
		}

		_ = os.Mkdir("results/", 0755)

		fd, err := os.Create(fmt.Sprintf("results/%s-%d_%d.yaml", contractAddr, startRound, endRound))
		if err != nil {
			log.Fatalf("Error creating file: %v", err)
		}
		defer fd.Close()

		writer := bufio.NewWriter(fd)
		defer writer.Flush()

		writeCount := 0
		first := true

		for res := range resultChan {
			if res.Err != nil {
				log.Warnf("Error from block %d: %v", res.StartBlock, res.Err)
				continue
			}
			if len(res.Output) == 0 {
				continue
			}

			if !first {
				if _, err := writer.Write([]byte("\n---\n")); err != nil {
					cfg.Error(err)
				}
			}
			first = false

			_, err := cfg.WithStdout(writer).Print(&config.Response{
				Result: res.Output,
			})

			if err != nil {
				cfg.Error(err)
			}

			writeCount++
			if writeCount%cfg.FlushEveryN == 0 {
				log.Debugf("flushing %d records", writeCount)
				if err := writer.Flush(); err != nil {
					cfg.Error(fmt.Errorf("Error flushing buffer: %v", err))
				}
			}
		}

	},
}
