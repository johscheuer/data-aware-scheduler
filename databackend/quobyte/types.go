package quobyte

import (
	quobyteAPI "github.com/johscheuer/api"
	"github.com/johscheuer/data-aware-scheduler/databackend"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api/v1"
)

type QuobyteBackend struct {
	quobyteClient     *quobyteAPI.QuobyteClient
	quobyteMountpoint string
	inKubernetes      string
	namespace         string
	clientset         *kubernetes.Clientset
}

var _ databackend.DataBackend = &QuobyteBackend{}

type segment struct {
	startOffset int
	length      int
	stripe      *stripe
}

type stripe struct {
	version   int
	deviceIDs []uint64
}

type device struct {
	id         uint64
	host       string // Fetch from Quobyte API
	dataSize   uint64 // TODO use BigInt?
	deviceType string // Fetch from Quobyte API -> SSD/HDD
	node       v1.Node
}

type deviceList []*device

func (devices deviceList) Len() int {
	return len(devices)
}

func (devices deviceList) Less(i, j int) bool {
	return devices[i].dataSize < devices[j].dataSize
}

func (devices deviceList) Swap(i, j int) {
	devices[i], devices[j] = devices[j], devices[i]
}
