package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus"
	//"crypto/tls"
	"github.com/oliveagle/jsonpath"
	//"io/ioutil"
	"encoding/json"
	"github.com/ovh/go-ovh/ovh"
	"github.com/go-errors/errors"
)

var addr = flag.String("web.listen-address", ":9116", "The address to listen on for HTTP requests.")

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
            <head><title>Json Exporter</title></head>
            <body>
            <h1>Json Exporter</h1>
            <p><a href="/probe">Run a probe</a></p>
            <p><a href="/metrics">Metrics</a></p>
            </body>
            </html>`))
	})
	flag.Parse()
	http.HandleFunc("/probe", probeHandler)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func doOvhProbe(target string) (interface{}, error){
	ovhClient, err :=ovh.NewEndpointClient("ovh-eu")
	
	if err != nil {
		fmt.Println(err.(*errors.Error).ErrorStack())
		return nil, err
	}
	
	// call get function
	bytes := []byte{}
	err = ovhClient.Get(target, &bytes)
	if err != nil {
		fmt.Println(err.(*errors.Error).ErrorStack())
		return nil, err
	}
	var jsonData interface{}
	err = json.Unmarshal([]byte(bytes), &jsonData)
	if err != nil {
		fmt.Println(err.(*errors.Error).ErrorStack())
		return nil, err
	}

	return jsonData, nil
}

func probeHandler(w http.ResponseWriter, r *http.Request) {

	params := r.URL.Query()
	target := params.Get("target")
	if target == "" {
		http.Error(w, "Target parameter is missing", 400)
		return
	}
	lookuppath := params.Get("jsonpath")
	if target == "" {
		http.Error(w, "The JsonPath to lookup", 400)
		return
	}
	probeSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_success",
		Help: "Displays whether or not the probe was a success",
	})
	probeDurationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_duration_seconds",
		Help: "Returns how long the probe took to complete in seconds",
	})
	valueGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:	"value",
			Help:	"Retrieved value",
		},
	)
	registry := prometheus.NewRegistry()
	registry.MustRegister(probeSuccessGauge)
	registry.MustRegister(probeDurationGauge)
	registry.MustRegister(valueGauge)

	jsonData, err := doOvhProbe(target)
	if err != nil {
		fmt.Println(err.(*errors.Error).ErrorStack())
		http.Error(w, "Unable to call OVH API", http.StatusInternalServerError)
		return
	} else {
		res, err := jsonpath.JsonPathLookup(jsonData, lookuppath)
		if err != nil {
			http.Error(w, "Jsonpath not found", http.StatusNotFound)
			return
		}
		log.Printf("Found value %v", res)
		number, ok := res.(float64)
		if !ok {
			http.Error(w, "Values could not be parsed to Float64", http.StatusInternalServerError)
			return
		}
		probeSuccessGauge.Set(1)
		valueGauge.Set(number)
	}

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
