package quobyte

import (
	"log"
	"path"
	"sort"

	"github.com/davecheney/xattr"
	quobyteAPI "github.com/johscheuer/api"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/labels"
)

func ifEmptySetDefault(m map[string]string, key string, defaultString string) string {
	if v, ok := m[key]; ok {
		return v
	}
	log.Printf("Missing %[1]s in opts using default %[1]s '%[2]s'\n", key, defaultString)

	return defaultString
}

func NewQuobyteBackend(opts map[string]string, clientset *kubernetes.Clientset) *QuobyteBackend {
	apiServer := ifEmptySetDefault(opts, "apiserver", "http://localhost:7860")
	if err := validateAPIURL(apiServer); err != nil {
		log.Fatalln(err)
	}

	return &QuobyteBackend{
		quobyteClient: quobyteAPI.NewQuobyteClient(
			apiServer,
			ifEmptySetDefault(opts, "user", "admin"),
			ifEmptySetDefault(opts, "password", "quobyte"),
		),
		quobyteMountpoint: ifEmptySetDefault(opts, "mountpoint", "/var/lib/kubelet/plugins/kubernetes.io~quobyte"),
		inKubernetes:      ifEmptySetDefault(opts, "kubernetes", ""),
		namespace:         ifEmptySetDefault(opts, "namespace", "quobyte"),
		clientset:         clientset,
	}
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
	log.Println("Get All Devices")
	devices := getDevices(filePath)

	// Get Quobyte DeviceEndpoints -> where is data located (on which Node)
	log.Println("Get All Device Details")
	if err := getDeviceDetails(quobyteBackend.quobyteClient, devices); err != nil {
		log.Println(err)
	}

	// TODO -> option in-kubernetes: True not as String
	if len(quobyteBackend.inKubernetes) > 0 {
		log.Println("Resolve Pod IPs")
		if err := quobyteBackend.resolvePodIPToNodeIP(devices, quobyteBackend.namespace); err != nil {
			log.Println(err)
		}
	}

	// Filter all nodes containing no devices
	log.Println("Get potentials nodes")
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

func (quobyteBackend *QuobyteBackend) resolvePodIPToNodeIP(devices deviceList, namespace string) error {
	podList, err := quobyteBackend.clientset.Core().Pods(namespace).List(
		api.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{"role": "data"}),
		})
	if err != nil {
		return err
	}

	for _, pod := range podList.Items {
		// TODO iterarte only over not resolved Devices
		for _, dev := range devices {
			if dev.host == pod.Status.PodIP {
				dev.host = pod.Status.HostIP

				log.Printf("Replace Pod IP %s with Host IP %s for Device %d\n", pod.Status.PodIP, pod.Status.HostIP, dev.id)
			}
		}

	}

	return nil
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
