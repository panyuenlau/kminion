package main

import (
	"fmt"
	"github.com/google-cloud-tools/kafka-minion/collector"
	"github.com/google-cloud-tools/kafka-minion/kafka"
	"github.com/google-cloud-tools/kafka-minion/options"
	"github.com/google-cloud-tools/kafka-minion/storage"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
)

func main() {
	// Initialize logger
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.JSONFormatter{})

	// Parse and validate environment variables
	opts := options.NewOptions()
	var err error
	err = envconfig.Process("", opts)
	if err != nil {
		log.Fatal("Error parsing env vars into opts. ", err)
	}

	// Set log level from environment variables
	level, err := log.ParseLevel(opts.LogLevel)
	if err != nil {
		log.Panicf("Loglevel could not be parsed. See logrus documentation for valid log level inputs. Given input was '%v'", opts.LogLevel)
	}
	log.SetLevel(level)

	log.Infof("Starting kafka minion version%v", opts.Version)
	// Create cross package shared dependencies
	consumerOffsetsCh := make(chan *kafka.ConsumerPartitionOffset, 1000)
	partitionWaterMarksCh := make(chan *kafka.PartitionWaterMarks, 200)

	// Create storage module
	cache := storage.NewOffsetStorage(consumerOffsetsCh, partitionWaterMarksCh)
	cache.Start()

	// Create cluster module
	cluster := kafka.NewCluster(opts, partitionWaterMarksCh)
	cluster.Start()

	// Create kafka consumer
	consumer := kafka.NewOffsetConsumer(opts, consumerOffsetsCh)
	consumer.Start()

	// Create prometheus collector
	collector := collector.NewCollector(opts, cache)
	prometheus.MustRegister(collector)

	// Start listening on /metrics endpoint
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/healthcheck", healthcheck())
	listenAddress := fmt.Sprintf(":%d", opts.Port)
	log.Infof("Listening on: '%s", listenAddress)
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}

func healthcheck() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Status: Healthy"))
	})
}
