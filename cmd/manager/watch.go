package manager

import (
	"chainlink-ocr-checker/config"
	"chainlink-ocr-checker/intra"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
	cli "github.com/spf13/cobra"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var contractRegex = regexp.MustCompile(`contract (0x[a-fA-F0-9]{40})`)

var watchCmd = &cli.Command{
	Use:     "watch",
	Aliases: []string{"w"},
	Example: "ocr-checker watch [transmitter1,transmitter2,...] [last_N] [last_Day_N]",
	Short:   "Watch Transmitter(s) across OCR2 jobs",
	Long:    `Watch if specific Transmitter(s) exist in recent OCR2 rounds`,
	Args:    cli.RangeArgs(1, 3),
	Run: func(cmd *cli.Command, args []string) {
		var (
			transmitterInputs = strings.Split(args[0], ",")
			transmitters      []common.Address
			lastCheckRound    = 5
			lastDayN          = 7
			err               error
		)

		for _, input := range transmitterInputs {
			transmitters = append(transmitters, common.HexToAddress(strings.TrimSpace(input)))
		}

		log.Infof("🔍 Watching %d Transmitter(s): %v", len(transmitters), transmitterInputs)

		if len(args) >= 2 {
			lastCheckRound, err = strconv.Atoi(args[1])
			if err != nil {
				cfg.Error(fmt.Errorf("❌ failed to parse last check round: %w", err))
			}

			if len(args) >= 3 {
				lastDayN, err = strconv.Atoi(args[2])
				if err != nil {
					cfg.Error(fmt.Errorf("❌ failed to parse last check days: %w", err))
				}
			}
		}

		ocr2Jobs, err := cfg.Repository.FindOCR2Jobs()
		if err != nil {
			cfg.Error(fmt.Errorf("❌ failed to fetch OCR2 jobs: %w", err))
		}

		var (
			wg       sync.WaitGroup
			results  = make(chan JobResult, len(ocr2Jobs)*len(transmitters))
			now      = time.Now()
			timeSpan = time.Hour * 24 * time.Duration(lastDayN)
		)

		for i := 0; i < len(ocr2Jobs); i++ {
			job := ocr2Jobs[i]
			if job.Name == nil {
				log.Warnf("⚠️ Job has no name (ID: %d), skipping", job.ID)
				continue
			}

			matches := contractRegex.FindStringSubmatch(*job.Name)
			if len(matches) < 2 {
				log.Warnf("⚠️ No contract address found in: %s", *job.Name)
				continue
			}

			contractAddr := matches[1]

			for _, transmitter := range transmitters {
				wg.Add(1)
				go func(contractAddr, jobName string, transmitter common.Address) {
					defer wg.Done()

					result, err := intra.FetchLatestN(cfg.Network, common.HexToAddress(contractAddr), lastCheckRound, QUERY_WINDOW)
					if err != nil {
						log.Error(err.Error())
						results <- JobResult{Status: ErrorJobStatus, Job: jobName, Transmitter: transmitter.Hex()}
						return
					}

					isActive := false
					observed := false
					inCharge := false

					for _, r := range result.Output {
						if r.Timestamp.After(now.Add(-timeSpan)) {
							isActive = true
						}

						for _, trans := range r.Transmitters {
							if strings.EqualFold(trans.Address.Hex(), transmitter.Hex()) {
								inCharge = true
							}
						}

						for _, ob := range r.Observers {
							if strings.EqualFold(ob.Address.Hex(), transmitter.Hex()) {
								observed = true
								break
							}
						}

					}

					if !isActive {
						results <- JobResult{Status: StaleJobStatus, Job: jobName, Transmitter: transmitter.Hex()}
						return
					}

					if !inCharge {
						results <- JobResult{Status: NoActiveJobStatus, Job: jobName, Transmitter: transmitter.Hex()}
						return
					}

					if !observed {
						results <- JobResult{Status: MissingJobStatus, Job: jobName, Transmitter: transmitter.Hex()}
					} else {
						results <- JobResult{Status: FoundJobStatus, Job: jobName, Transmitter: transmitter.Hex()}
					}
				}(contractAddr, *job.Name, transmitter)
			}
		}

		wg.Wait()
		close(results)

		var jobResults []JobResult
		for res := range results {
			jobResults = append(jobResults, res)
		}

		_, err = cfg.Print(&config.Response{
			Result: jobResults,
		})
		if err != nil {
			cfg.Error(err)
		}
	},
}

type JobStatus string

var (
	ErrorJobStatus    = JobStatus("ERROR_JOB_STATUS")
	StaleJobStatus    = JobStatus("STALE_JOB_STATUS")
	FoundJobStatus    = JobStatus("FOUND_JOB_STATUS")
	MissingJobStatus  = JobStatus("MISSING_JOB_STATUS")
	NoActiveJobStatus = JobStatus("NO_ACTIVE_JOB_STATUS")
)

type JobResult struct {
	Status      JobStatus `json:"status"`
	Job         string    `json:"job"`
	Transmitter string    `json:"transmitter"`
}
