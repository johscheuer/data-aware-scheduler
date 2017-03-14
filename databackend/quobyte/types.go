package quobyte

import (
	quobyteAPI "github.com/johscheuer/api"
	"github.com/johscheuer/data-aware-scheduler/databackend"
	"k8s.io/client-go/1.5/kubernetes"
)

type QuobyteBackend struct {
	quobyteClient     *quobyteAPI.QuobyteClient
	quobyteMountpoint string
	inKubernetes      bool
	namespace         string
	clientset         *kubernetes.Clientset
	nodeCache         map[string]string
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
}

type deviceList []*device

type quobyteInput struct {
	files []string
	dir   string
}
