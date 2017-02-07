package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api/v1"

	"github.com/johscheuer/data-aware-scheduler/databackend"
)

type processor struct {
	clientset     *kubernetes.Clientset
	wg            *sync.WaitGroup
	done          chan struct{}
	processorLock *sync.Mutex
	databackend.DataBackend    *databackend
}

func newProcessor(clientset *kubernetes.Clientset, done chan struct{}, wg *sync.WaitGroup, databackend *databackend.DataBackend) *processor {
	return &processor{
		processorLock: &sync.Mutex{},
		wg:            wg,
		done:          done,
		clientset:     clientset,
		databackend:   databackend,
	}
}

func (p *processor) reconcileUnscheduledPods(interval int) {
	for {
		select {
		case <-time.After(time.Duration(interval) * time.Second):
			if err := p.schedulePods(); err != nil {
				log.Println(err)
			}
		case <-p.done:
			p.wg.Done()
			log.Println("Stopped reconciliation loop.")
			return
		}
	}
}

func (p *processor) monitorUnscheduledPods() {
	pods, errc := watchUnscheduledPods(p.clientset)

	for {
		select {
		case err := <-errc:
			log.Println(err)
		case pod := <-pods:
			p.processorLock.Lock()
			time.Sleep(2 * time.Second)
			if err := p.schedulePod(pod); err != nil {
				log.Println(err)
			}
			p.processorLock.Unlock()
		case <-p.done:
			p.wg.Done()
			log.Println("Stopped scheduler.")
			return
		}
	}
}

func (p *processor) schedulePod(pod *v1.Pod) error {
	nodes, err := fit(p.clientset, pod)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		return fmt.Errorf("Unable to schedule pod (%s) failed to fit in any node", pod.ObjectMeta.Name)
	}

	node, err := p.databackend.GetBestFittingNode(nodes, pod)
	if err != nil {
		return err
	}

	return bind(p.clientset, pod, node)
}

func (p *processor) schedulePods() error {
	p.processorLock.Lock()
	defer p.processorLock.Unlock()
	pods, err := getUnscheduledPods(p.clientset)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		if err := p.schedulePod(pod); err != nil {
			log.Println(err)
		}
	}

	return nil
}
