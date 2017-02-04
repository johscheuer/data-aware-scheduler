package main

import (
	"github.com/johscheuer/data-aware-scheduler/databackend"
	"k8s.io/client-go/1.5/pkg/api/v1"
)

type dataLocator struct {
	dataBackend databackend.DataBackend
}

func newDataLocator(dataBackend databackend.DataBackend) *dataLocator {
	return &dataLocator{
		dataBackend: dataBackend,
	}
}

func (d *dataLocator) findNode(nodes []v1.Node, pod *v1.Pod) (v1.Node, error) {
	return d.dataBackend.GetBestFittingNode(nodes, pod)
}
