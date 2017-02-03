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
	version   int
	deviceIDs []uint64
}

type device struct {
	id         uint64
	host       string // Fetch from Quobyte API
	dataSize   uint64 // TODO use BigInt?
	deviceType string // Fetch from Quobyte API -> SSD/HDD
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
// 3.) Allow multiple files and find best fitting node
func (d *dataLocator) findNode(nodes []v1.Node, pod *v1.Pod) (v1.Node, error) {
	var file string
	var volume string
	var diskType string

	if f, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/file"]; ok {
		// Operator needs to tell us which file(s) should be considered
		file = f
	}

	if v, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/volume"]; ok {
		// If there are more than one Quobyte Volume specified we need some help
		volume = v
	}

	if d, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/type"]; ok {
		// Optional
		diskType = d
		_ = diskType
	}
	// parse podSpec for quobyte Mounts if there are non choose random node
	// Todo check if file and/or volume is suplied else return random node

	// Get Quobyte metadata -> where is data located
	device := getDevices(fmt.Sprintf("%s/%s/%s", d.quobyteMountpoint, volume, file))

	// Ask Quobyte API Where Device are located (on which Node)
	if err := d.getDeviceDetails(device); err != nil {
		log.Println(err)
	}

	// TODO Filter here if there are Devices located on potential Nodes
	// if not choose a node randomly
	// if a device is not located on a node drop it

	// TODO check if quobyte runs in cluster -> mapping between Pod IP <-> Node IP
	// resolve Pod name to node name (if Quobyte runs containerized)
	// else we can take Node IP

	// TODO calculate best fitting (biggest segment?)
	// node := getBestFittingDevice().host

	// Assumption Quobyte + Kubernetes runs on all Nodes
	return nodes[0], nil
}

func getDevices(filePath string) map[uint64]*device {
	b, err := xattr.Getxattr(filePath, "quobyte.info")
	if err != nil {
		log.Println(err)
	}

	segments := parseXattrSegments(string(b))
	for s := range segments {
		log.Println(s)
	}

	return convertSegmentsToDevices(segments)
}

func convertSegmentsToDevices(segments []*segment) map[uint64]*device {
	result := map[uint64]*device{}

	// Go over all Segments -> strips
	for _, seg := range segments {
		for _, devID := range seg.stripe.deviceIDs {
			if d, ok := result[devID]; ok {
				d.dataSize += uint64(seg.length)
			} else {
				result[devID] = &device{
					id:         devID,
					host:       "",
					dataSize:   uint64(seg.length),
					deviceType: "", // Fetch from NetworkEndpoints
				}
			}
		}
	}

	return result
}

func getBestFittingDevice(devices map[uint64]*device) *device {
	// TODO first we take the first ID in the first segment
	// TODO next step merge all segments and find biggest chunk

	// we actually need to sort them by fitness
	for _, dev := range devices {
		_ = dev
		//todo sort strips and device id's
	}

	// We could check here also DeviceType -> e.q. Fast Data (SSD) / Disk Capacity (HDD)

	return &device{}
}

func (d *dataLocator) getDeviceDetails(devices map[uint64]*device) error {
	//TODO copy list here ?
	for _, dev := range devices {
		endpoints, err := d.quobyteClient.GetDeviceNetworkEndpoints(dev.id)
		if err != nil {
			// TODO -> better error handling
			log.Println(err)
			continue
		}

		dev.host = endpoints.Endpoints[0].Hostname
		dev.deviceType = endpoints.Endpoints[0].DeviceType
	}

	return nil
}
