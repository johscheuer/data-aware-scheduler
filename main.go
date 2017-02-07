package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/tools/clientcmd"

	"github.com/johscheuer/data-aware-scheduler/databackend/quobyte"
)

var (
	schedulerConfigPath = flag.String("config", "./config.yaml", "absolute path to the scheduler config file")
)

const schedulerName = "data-aware-scheduler"

func main() {
	log.Println("Starting data-aware-scheduler scheduler...")

	flag.Parse()

	schedulerConfig := readConfig(*schedulerConfigPath)

	config, err := clientcmd.BuildConfigFromFlags("", schedulerConfig.Kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	doneChan := make(chan struct{})
	var wg sync.WaitGroup

	var dataBackend *dataBackend.Databackend
	if schedulerConfig.Backend == "quobyte" {
		dataBackend = quobyte.NewQuobyteBackend(
			schedulerConfig.Opts,
			clientset,
		)
	}

	processor := newProcessor(clientset, doneChan, &wg, dataBackend)
	wg.Add(1)
	go processor.monitorUnscheduledPods()

	wg.Add(1)
	go processor.reconcileUnscheduledPods(30)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-signalChan:
			log.Printf("Shutdown signal received, exiting...")
			close(doneChan)
			wg.Wait()
			os.Exit(0)
		}
	}
}
