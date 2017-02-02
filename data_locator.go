package main

import (
	"fmt"
	"log"

	"github.com/davecheney/xattr"
	quobyte_api "github.com/johscheuer/api"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api/v1"
)

type dataLocator struct {
	quobyteClient     *quobyte_api.QuobyteClient
	quobyteMountpoint string
	clientset         *kubernetes.Clientset
}

type segment struct {
	startOffset int
	length      int
	stripe      *stripe
}

type stripe struct {
	version    int
	device_ids []uint64
}

func newDataLocator(quobyteAPIServer string, quobyteUser string, quobytePassword string, quobyteMountpoint string, clientset *kubernetes.Clientset) *dataLocator {
	return &dataLocator{
		quobyteClient:     quobyte_api.NewQuobyteClient(quobyteAPIServer, quobyteUser, quobytePassword),
		quobyteMountpoint: quobyteMountpoint,
		clientset:         clientset,
	}
}

// Scheduling types ->
// 1.) data locality
// 2.) i/o rate (SSD/HDD)
func (d *dataLocator) findNode(nodes []v1.Node, pod *v1.Pod) (v1.Node, error) {
	var file string
	var volume string

	if f, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/file"]; ok {
		// Operator needs to tell us which file(s) should be considered
		file = f
	}

	if v, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/volume"]; ok {
		// If there are more than one Quobyte Volume specified we need some help
		volume = v
	}
	// parse podSpec for quobyte Mounts if there are non choose random node
	// Todo check if file and/or volume is suplied else return random node

	// Get Quobyte metadata -> where is data located
	device := getBestFittingDevice(fmt.Sprintf("%s/%s/%s", d.quobyteMountpoint, volume, file))
	// Ask Quobyte API Where Device are located (on which Node)
	node, err := d.getHostOfDevice(device)
	if err != nil {
		log.Println(err)
	}

	//TODO check if quobyte runs in cluster -> mapping between Pod IP <-> Node IP
	// resolve Pod name to node name (if Quobyte runs containerized)
	// else we can take Node IP

	// Check if node is in node list else take another one

	// not included in list just use a random one

	// Assumption Quobyte + Kubernetes runs on all Nodes
	return nodes[0], nil
}

func getBestFittingDevice(filePath string) uint64 {
	b, err := xattr.Getxattr(filePath, "quobyte.info")
	if err != nil {
		log.Println(err)
	}

	segments := parseXattrSegments(string(b))
	for s := range segments {
		log.Println(s)
	}

	// we actually need to sort them by fitness
	return findBestFit(segments)
}

func findBestFit(segments []*segment) uint64 {
	// TODO first we take the first ID in the first segment
	// TODO next step merge all segments and find biggest chunk
	return 0
}

func (d *dataLocator) getHostOfDevice(device_id uint64) (string, error) {
	endpoints, err := d.quobyteClient.GetDeviceNetworkEndpoints(uint64(device_id))
	if err != nil {
		return "", err
	}

	// We could check here also DeviceType -> e.q. Fast Data (SSD) / Disk Capacity (HDD)
	return endpoints.Endpoints[0].Hostname, nil
}
