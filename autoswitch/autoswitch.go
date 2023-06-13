package autoswitch

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// AlgoShortnames used to build the whattomine uri
var AlgoShortnames = map[string]string{
	"beamv3":      "eqb",
	"cuckoocycle": "cc",
	"cuckatoo32":  "ct32",
	"etchash":     "etc",
	"ethash":      "eth",
	"kawpow":      "kpw",
	"kheavyhash":  "hh",
	"octopus":     "ops",
	"zelhash":     "zlh",
	"zhash":       "zh",
}

// GpuShortnames used to build the whattomine uri
var GpuShortnames = map[string]string{
	"amd69xt":   "69xt",
	"amd68xt":   "68xt",
	"amd67xt":   "67xt",
	"amd66xt":   "66xt",
	"vii":       "vii",
	"amd5700xt": "5700xt",
	"amd5700":   "5700",
	"amd5600xt": "5600xt",
	"vega64":    "vega64",
	"vega56":    "vega56",
	"nvi4090":   "4090",
	"nvi4080":   "4080",
	"nvi47Ti":   "47Ti",
	"nvi47":     "47",
	"nvi39Ti":   "39Ti",
	"nvi3090":   "3090",
	"nvi38Ti":   "38Ti",
	"nvi3080":   "3080",
	"nvi37Ti":   "37Ti",
	"nvi3070":   "3070",
}

type Config struct {
	Gpus    map[string]int       `yaml:"gpus"`
	Algos   map[string]Algorithm `yaml:"algos"`
	General General              `yaml:"general"`
}

type Switcher struct {
	Config *Config
}

func (s *Switcher) GetURI(c context.Context) (string, error) {
	uri := "https://whattomine.com/coins?"

	for gpu, count := range s.Config.Gpus {
		var gpuStr string
		gpuCode := GpuShortnames[gpu]
		if count != 0 {
			gpuStr = "aq_" + gpuCode + "=" + strconv.Itoa(count) + "&a_" + gpuCode + "=true&"
		} else {
			gpuStr = "aq_" + gpuCode + "=0&"
		}
		uri = uri + gpuStr
	}

	for algo := range s.Config.Algos {
		algoCode := AlgoShortnames[algo]
		hashRate := strconv.Itoa(s.Config.Algos[algo].HashRate)
		power := strconv.Itoa(s.Config.Algos[algo].Power)
		algoStr := "&" + algoCode + "=true&factor%5B" + algoCode + "_hr%5D=" + hashRate + "&factor%5B" + algoCode + "_p%5D=" + power
		uri = uri + algoStr
	}

	costStr := "&factor%5Bcost%5D=" + fmt.Sprintf(
		"%f",
		s.Config.General.PowerCostPerKwh,
	) + "&factor%5Bcost_currency%5D+USD&sort=Revenue&volume=0&revenue=24h&factor%5Bexchanges%5D%5B%5D=&factor%5Bexchanges%5D%5B%5D=binance&factor%5Bexchanges%5D%5B%5D=bitfinex&factor%5Bexchanges%5D%5B%5D=bitforex&factor%5Bexchanges%5D%5B%5D=bittrex&factor%5Bexchanges%5D%5B%5D=coinex&factor%5Bexchanges%5D%5B%5D=exmo&factor%5Bexchanges%5D%5B%5D=gate&factor%5Bexchanges%5D%5B%5D=graviex&factor%5Bexchanges%5D%5B%5D=hitbtc&factor%5Bexchanges%5D%5B%5D=ogre&factor%5Bexchanges%5D%5B%5D=poloniex&factor%5Bexchanges%5D%5B%5D=stex&dataset=Main&commit=Calculate"
	uri = uri + costStr

	return uri, nil
}

func (s *Switcher) GetBestAlgo(c context.Context) (string, error) {
	uri, err := s.GetURI(c)
	if err != nil {
		return "", err
	}

	resp, err := http.Get(uri)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// write output to file
	file, err := os.Create("output.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	_, err = fmt.Fprint(file, doc.Text())
	if err != nil {
		log.Fatal(err)
	}
	f, _ := os.Open("output.txt")

	// Scan file for 1st nicehash occurence.
	scanner := bufio.NewScanner(f)
	found := false
	var line string

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "Nicehash-") && !found {
			found = true
			line = scanner.Text()
		}
	}
	splittedLine := strings.Split(line, "-")
	algo := strings.ToLower(strings.TrimSuffix(splittedLine[1], "<br>"))
	return algo, nil
}
