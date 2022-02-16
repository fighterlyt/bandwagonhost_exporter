package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

var (
	configPath = flag.String(`config`, `config.yml`, `config file path`)
)

func main() {
	flag.Parse()

	logger := log.Default()
	dataGauge := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: `bandwagon`,
		Name:      `dataGauge`,
		Help:      `data Usage Gauge,label include instance dataKind(plan plan data in bytes,used used data in bytes,renew renew time in epoch)`,
	}, []string{`instance`, `dataKind`})

	config, err := decodeConfig(*configPath)

	if err != nil {
		panic(err.Error())
	}

	client := &http.Client{}

	for _, host := range config.Hosts {
		go func(host HostConfig) {
			defer func() {
				x := recover()
				if x != nil {
					logger.Printf(`%v`, x)
				}
			}()

			for ; ; time.Sleep(config.ScrapeInterval) {
				getData(host, dataGauge, client, logger)
			}
		}(host)
	}

	http.Handle("/metrics", promhttp.Handler())

	if config.Port == 0 {
		config.Port = 9131
	}

	logger.Fatal(http.ListenAndServe(fmt.Sprintf(`:%d`, config.Port), nil))
}

func decodeConfig(filePath string) (config *Config, err error) {
	var (
		file *os.File
	)

	if file, err = os.Open(filePath); err != nil {
		return nil, errors.Wrap(err, `open config file`)
	}

	defer func() {
		_ = file.Close()
	}()

	config = &Config{}

	if err = yaml.NewDecoder(file).Decode(config); err != nil {
		return nil, errors.Wrap(err, `decode config file`)
	}

	return config, nil
}

func getData(config HostConfig, gaugeVec *prometheus.GaugeVec, client *http.Client, logger *log.Logger) {
	hostURL := fmt.Sprintf(`https://api.64clouds.com/v1/getServiceInfo?veid=%s&api_key=%s`, config.Veid, config.APIKey)

	var (
		resp     *http.Response
		err      error
		respBody []byte
		result   = &data{}
	)

	if resp, err = client.Get(hostURL); err != nil {
		logger.Printf(`Error:%s\n`, err.Error())
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if respBody, err = io.ReadAll(resp.Body); err != nil {
		logger.Printf(`ReadAll Error:%s\n`, err.Error())
		return
	}

	if err = json.NewDecoder(bytes.NewReader(respBody)).Decode(result); err != nil {
		logger.Printf(`Decode Error:%s Data:[%s]\n`, err.Error(), string(respBody))
		return
	}

	if result.Error != 0 {
		logger.Printf(`Fail: %d\n`, result.Error)
		return
	}

	gaugeVec.WithLabelValues(config.Name, `plan`).Set(float64(result.PlanMonthlyData * result.MonthlyDataMultiplier))
	gaugeVec.WithLabelValues(config.Name, `used`).Set(float64(result.DataCounter * result.MonthlyDataMultiplier))
	gaugeVec.WithLabelValues(config.Name, `renew`).Set(float64(result.DataNextReset))
	gaugeVec.WithLabelValues(config.Name, `last`).Set(float64(time.Now().Unix()))
}

type data struct {
	VmType                          string        `json:"vm_type"`
	Hostname                        string        `json:"hostname"`
	NodeIp                          string        `json:"node_ip"`
	NodeAlias                       string        `json:"node_alias"`
	NodeLocation                    string        `json:"node_location"`
	NodeLocationId                  string        `json:"node_location_id"`
	NodeDatacenter                  string        `json:"node_datacenter"`
	LocationIpv6Ready               bool          `json:"location_ipv6_ready"`
	Plan                            string        `json:"plan"`
	PlanMonthlyData                 int64         `json:"plan_monthly_data"`       // 月套餐流量
	MonthlyDataMultiplier           int64         `json:"monthly_data_multiplier"` // 月套餐流量系数
	PlanDisk                        int64         `json:"plan_disk"`
	PlanRam                         int64         `json:"plan_ram"`
	PlanSwap                        int           `json:"plan_swap"`
	PlanMaxIpv6S                    int           `json:"plan_max_ipv6s"`
	Os                              string        `json:"os"`
	Email                           string        `json:"email"`
	DataCounter                     int64         `json:"data_counter"`    // 已使用流量
	DataNextReset                   int           `json:"data_next_reset"` // 流量重置时间
	IpAddresses                     []string      `json:"ip_addresses"`
	PrivateIpAddresses              []interface{} `json:"private_ip_addresses"`
	IpNullroutes                    []interface{} `json:"ip_nullroutes"`
	Iso1                            interface{}   `json:"iso1"`
	Iso2                            interface{}   `json:"iso2"`
	AvailableIsos                   []string      `json:"available_isos"`
	PlanPrivateNetworkAvailable     bool          `json:"plan_private_network_available"`
	LocationPrivateNetworkAvailable bool          `json:"location_private_network_available"`
	RdnsApiAvailable                bool          `json:"rdns_api_available"`
	Ptr                             struct {
		Field1 interface{} `json:"45.62.121.205"`
	} `json:"ptr"`
	Suspended        bool        `json:"suspended"`
	PolicyViolation  bool        `json:"policy_violation"`
	SuspensionCount  interface{} `json:"suspension_count"`
	TotalAbusePoints int         `json:"total_abuse_points"`
	MaxAbusePoints   int         `json:"max_abuse_points"`
	Error            int         `json:"error"`
}
