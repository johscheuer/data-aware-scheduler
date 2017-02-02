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
)

var (
	kubeconfig = flag.String("kubeconfig", "./config", "absolute path to the kubeconfig file")
)

const schedulerName = "data-aware-scheduler"

func main() {
	log.Println("Starting data-aware-scheduler scheduler...")

	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	doneChan := make(chan struct{})
	var wg sync.WaitGroup

	processor := newProcessor(clientset, doneChan, &wg, &dataLocator{})
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
