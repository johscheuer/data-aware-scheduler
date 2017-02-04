package quobyte

import (
	"fmt"
	"log"
	"path"
	"sort"

	"github.com/davecheney/xattr"
	quobyteAPI "github.com/johscheuer/api"
	"github.com/johscheuer/data-aware-scheduler/databackend"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api/v1"
)

type QuobyteBackend struct {
	quobyteClient     *quobyteAPI.QuobyteClient
	quobyteMountpoint string
	clientset         *kubernetes.Clientset
}

func NewQuobyteBackend(opts map[string]string, clientset *kubernetes.Clientset) *QuobyteBackend {
	var apiServer, user, password, mountpoint string

	if u, ok := opts["user"]; ok {
		user = u
	} else {
		user = "admin"
		log.Println("Missing User in opts using default user 'admin'")
	}

	if p, ok := opts["password"]; ok {
		password = p
	} else {
		password = "quobyte"
		log.Println("Missing password in opts using default password 'quobyte'")
	}

	if a, ok := opts["apiserver"]; ok {
		apiServer = a
		if err := validateAPIURL(apiServer); err != nil {
			log.Fatalln(err)
		}
	} else {
		apiServer = "http://localhost:7860"
		log.Println("Missing API Server in opts using default apiServer 'http://localhost:7860'")
	}

	if m, ok := opts["mountpoint"]; ok {
		mountpoint = m
	} else {
		mountpoint = "/var/lib/kubelet/plugins/kubernetes.io~quobyte"
		log.Println("Missing Mountpoint in opts using default mountpoint '/var/lib/kubelet/plugins/kubernetes.io~quobyte'")
	}

	return &QuobyteBackend{
		quobyteClient:     quobyteAPI.NewQuobyteClient(apiServer, user, password),
		quobyteMountpoint: mountpoint,
		clientset:         clientset,
	}
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

type QuobyteMountNotFound struct {
	Volume string
}

func (e *QuobyteMountNotFound) Error() string {
	return fmt.Sprintf("Error: Volume Mount for Volume %s not found in PodSpec\n", e.Volume)
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

func getDevices(filePath string) deviceList {
	b, err := xattr.Getxattr(filePath, "quobyte.info")
	if err != nil {
		log.Println(err)
	}

	return convertSegmentsToDevices(parseXattrSegments(string(b)))
}

func convertSegmentsToDevices(segments []*segment) deviceList {
	result := map[uint64]*device{}

	for _, seg := range segments {
		for _, devID := range seg.stripe.deviceIDs {
			if d, ok := result[devID]; ok {
				d.dataSize += uint64(seg.length)
			} else {
				result[devID] = &device{
					id:         devID,
					host:       "", // Fetch from NetworkEndpoints
					dataSize:   uint64(seg.length),
					deviceType: "", // Fetch from NetworkEndpoints
				}
			}
		}
	}

	return convertDeviceMapIntoSlice(result)
}

func convertDeviceMapIntoSlice(devices map[uint64]*device) deviceList {
	res := make(deviceList, len(devices))

	i := 0
	for _, value := range devices {
		res[i] = value
		i++
	}

	return res
}

// Scheduling types ->
// 1.) data locality -> works
// 2.) i/o rate (SSD/HDD)
// 3.) Allow multiple files and find best fitting node
func (quobyteBackend *QuobyteBackend) GetBestFittingNode(nodes []v1.Node, pod *v1.Pod) (v1.Node, error) {
	filePath, err := quobyteBackend.parsePodSpec(pod)
	if err != nil {
		return nodes[0], err
	}

	// Get all devices that store data of this file
	devices := getDevices(filePath)

	// Get Quobyte DeviceEndpoints -> where is data located (on which Node)
	if err := getDeviceDetails(quobyteBackend.quobyteClient, devices); err != nil {
		log.Println(err)
	}

	// TODO -> option in-kubernetes: True
	// check if quobyte runs in cluster -> mapping between Pod IP <-> Node IP
	// get all pods in namespace=quobyte role=data -> IP -> NodeStatus
	// resolve Pod name to node name (if Quobyte runs containerized)
	// else we can take Node IP

	// Filter all nodes containing no data
	devices = getDevicesOnPotentialNodes(devices, nodes)

	// We could check here also DeviceType
	// -> e.q. Fast Data (SSD) / Disk Capacity (HDD)
	// and implement smarter algos
	if len(devices) == 0 {
		log.Printf("No suitable Devices found on Nodes -> schedule on first Node in list %s\n", nodes[0].ObjectMeta.Labels["kubernetes.io/hostname"])
		return nodes[0], nil
	}

	log.Printf("Schedule pod on Node %s\n", devices[0].node.ObjectMeta.Labels["kubernetes.io/hostname"])
	return devices[0].node, nil
}

func getDevicesOnPotentialNodes(devices deviceList, nodes []v1.Node) deviceList {
	res := deviceList{}

	// O(Devices x Nodes)
	for _, dev := range devices {

		for _, node := range nodes {
			for _, addr := range node.Status.Addresses {
				if dev.host == addr.Address {
					dev.node = node
					res = append(res, dev)
					continue
				}
			}
		}
	}
	// Sort by Size
	sort.Sort(res)

	return res
}

func getDeviceDetails(quobyteClient *quobyteAPI.QuobyteClient, devices deviceList) error {
	for _, dev := range devices {
		endpoints, err := quobyteClient.GetDeviceNetworkEndpoints(dev.id)
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

func (quobyteBackend *QuobyteBackend) parsePodSpec(pod *v1.Pod) (string, error) {
	var file string
	var volume string
	var diskType string

	// TODO parse podSpec for quobyte Mounts if there are non
	// choose random node

	if f, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/file"]; ok {
		// Operator needs to tell us which file(s) should be considered
		file = f
	}

	if v, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/volume"]; ok {
		// If there are more than one Quobyte Volume specified we need some help
		// Otherwise we could parse it from PodSpec
		volume = v
	}

	if d, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/type"]; ok {
		// Optional
		diskType = d
		_ = diskType
	}

	return path.Join(quobyteBackend.quobyteMountpoint, volume, file), nil
}
