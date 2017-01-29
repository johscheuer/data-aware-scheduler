package main

import (
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api/v1"
)

func dataLocator(clientset *kubernetes.Clientset, nodes []v1.Node, pod *v1.Pod) (v1.Node, error) {
	// TODO get Quobyte metadata -> where is data located
	// parse podSpec for quobyte Mounts if there are non choose random node
	// resolve Pod name to node name (if Quobyte runs containerized)
	return nodes[0], nil
}
