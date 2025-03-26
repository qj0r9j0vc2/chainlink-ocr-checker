package manager

import (
	"chainlink-ocr-checker/config"
	"fmt"
	"github.com/pkg/errors"
	cli "github.com/spf13/cobra"
	"io"
	"os"
	"sort"
	"strings"
)

type ParseUnit string

var (
	DayParseUnit   ParseUnit = "day"
	MonthParseUnit ParseUnit = "month"
)

var parseCmd = &cli.Command{
	Use:     "parse",
	Aliases: []string{"p"},
	Example: "ocr-checker parse [file] [unit; day,month]",
	Short:   "parse a file",
	Long:    `parse a file`,
	Args:    cli.ExactArgs(2),
	Run: func(cmd *cli.Command, args []string) {

		var (
			file      = args[0]
			parseUnit = ParseUnit(args[1])
		)

		if parseUnit != DayParseUnit && parseUnit != MonthParseUnit {
			cfg.Error(errors.Errorf("invalid unit: %s", parseUnit))
		}

		f, err := os.Open(file)
		if err != nil {
			cfg.Error(err)
		}
		defer f.Close()

		decoder := cfg.NewDecoder(f)

		var (
			observerResults []config.Result
		)

		for {
			var doc config.TransmissionsResponse
			if err := decoder.Decode(&doc); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				cfg.Error(errors.Wrap(err, "failed decoding YAML chunk"))
			}

			observerResults = append(observerResults, doc.Result...)
		}

		if len(observerResults) == 0 {
			cfg.Error(errors.New("no observers found"))
		}

		sort.Slice(observerResults, func(i, j int) bool {
			return observerResults[i].Timestamp.Before(observerResults[j].Timestamp)
		})

		result := make(map[string]map[string]uint64)

		for _, ores := range observerResults {
			for _, ob := range ores.Observers {
				addr := ob.Address.String()
				var timeKey string

				switch parseUnit {
				case DayParseUnit:
					timeKey = ores.Timestamp.Format("2006-01-02")
				case MonthParseUnit:
					timeKey = ores.Timestamp.Format("2006-01")
				default:
					continue
				}

				if result[addr] == nil {
					result[addr] = make(map[string]uint64)
				}
				result[addr][timeKey]++
			}
		}

		var resultKeys []string
		for addr := range result {
			resultKeys = append(resultKeys, addr)
		}
		sort.Strings(resultKeys)

		dateSet := make(map[string]struct{})
		for _, dateCount := range result {
			for date := range dateCount {
				dateSet[date] = struct{}{}
			}
		}

		var sortedDates []string
		for date := range dateSet {
			sortedDates = append(sortedDates, date)
		}
		sort.Strings(sortedDates)

		fmt.Printf("%-42s", "Observer Address")
		for _, date := range sortedDates {
			fmt.Printf("| %-10s ", date)
		}
		fmt.Println()
		fmt.Println(strings.Repeat("-", 42+len(sortedDates)*14))

		for _, addr := range resultKeys {
			fmt.Printf("%-42s", addr)
			for _, date := range sortedDates {
				count := result[addr][date]
				fmt.Printf("| %-10d ", count)
			}
			fmt.Println()
		}

	},
}
