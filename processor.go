package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api/v1"
)

var processorLock = &sync.Mutex{}

func reconcileUnscheduledPods(clientset *kubernetes.Clientset, interval int, done chan struct{}, wg *sync.WaitGroup) {
	for {
		select {
		case <-time.After(time.Duration(interval) * time.Second):
			err := schedulePods(clientset)
			if err != nil {
				log.Println(err)
			}
		case <-done:
			wg.Done()
			log.Println("Stopped reconciliation loop.")
			return
		}
	}
}

func monitorUnscheduledPods(clientset *kubernetes.Clientset, done chan struct{}, wg *sync.WaitGroup) {
	pods, errc := watchUnscheduledPods(clientset)

	for {
		select {
		case err := <-errc:
			log.Println(err)
		case pod := <-pods:
			processorLock.Lock()
			time.Sleep(2 * time.Second)
			if err := schedulePod(clientset, pod); err != nil {
				log.Println(err)
			}
			processorLock.Unlock()
		case <-done:
			wg.Done()
			log.Println("Stopped scheduler.")
			return
		}
	}
}

func schedulePod(clientset *kubernetes.Clientset, pod *v1.Pod) error {
	nodes, err := fit(clientset, pod)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		return fmt.Errorf("Unable to schedule pod (%s) failed to fit in any node", pod.ObjectMeta.Name)
	}

	node, err := dataLocator(clientset, nodes, pod)
	if err != nil {
		return err
	}

	return bind(clientset, pod, node)
}

func schedulePods(clientset *kubernetes.Clientset) error {
	processorLock.Lock()
	defer processorLock.Unlock()
	pods, err := getUnscheduledPods(clientset)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		if err := schedulePod(clientset, pod); err != nil {
			log.Println(err)
		}
	}

	return nil
}
