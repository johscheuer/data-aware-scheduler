package databackend

import "k8s.io/client-go/1.5/pkg/api/v1"

type DataBackend interface {
	GetBestFittingNode(nodes []v1.Node, pod *v1.Pod) (v1.Node, error)
}
