package quobyte

import (
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/davecheney/xattr"
	quobyteAPI "github.com/johscheuer/api"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/labels"
)

func ifEmptySetDefault(m map[string]interface{}, key string, defaultString string) string {
	if v, ok := m[key]; ok {
		return v.(string)
	}
	log.Printf("Missing %[1]s in opts using default %[1]s '%[2]s'\n", key, defaultString)

	return defaultString
}

func NewQuobyteBackend(opts map[string]interface{}, clientset *kubernetes.Clientset) *QuobyteBackend {
	var inKubernetes bool
	apiServer := ifEmptySetDefault(opts, "apiserver", "http://localhost:7860")
	if err := validateAPIURL(apiServer); err != nil {
		log.Fatalln(err)
	}
	if v, ok := opts["kubernetes"]; ok {
		inKubernetes = v.(bool)
	}

	return &QuobyteBackend{
		quobyteClient: quobyteAPI.NewQuobyteClient(
			apiServer,
			ifEmptySetDefault(opts, "user", "admin"),
			ifEmptySetDefault(opts, "password", "quobyte"),
		),
		quobyteMountpoint: ifEmptySetDefault(opts, "mountpoint", "/var/lib/kubelet/plugins/kubernetes.io~quobyte"),
		namespace:         ifEmptySetDefault(opts, "namespace", "quobyte"),
		clientset:         clientset,
		inKubernetes:      inKubernetes,
	}
}

func getSegmentsForFiles(files []string) []*segment {
	segments := []*segment{}
	for _, file := range files {
		log.Printf("Fetch xattr from %s\n", file)
		b, err := xattr.Getxattr(file, "quobyte.info")
		if err != nil {
			log.Printf("QuobyteBackend: Failed fetching Segements from file %s - %s\n", file, err)
			continue
		}
		segments = append(segments, parseXattrSegments(string(b))...)
	}

	return segments
}

// Get all devices that store data of this file
func (quobyteBackend *QuobyteBackend) getQuobyteDevices(input *quobyteInput) deviceList {
	log.Println("Get All Devices")
	var segments []*segment

	if input.dir != "" {
		files := getAllFilesInsideDir(input.dir)
		segments = append(segments, getSegmentsForFiles(files)...)
	}

	if len(input.files) > 0 {
		segments = append(segments, getSegmentsForFiles(input.files)...)
	}

	devices, requestIDs := convertSegmentsToDevices(segments)
	quobyteBackend.getDeviceDetails(devices, requestIDs)
	return convertDeviceMapIntoSlice(devices)
}

// Get Quobyte DeviceEndpoints -> where is data located (on which Node)
func (quobyteBackend *QuobyteBackend) getDeviceDetails(devices map[uint64]*device, requestIDs []uint64) {
	log.Printf("Get All Device Details for devices: %v\n", requestIDs)
	response, err := quobyteBackend.quobyteClient.GetDeviceList(requestIDs, []string{"DATA"})
	if err != nil {
		// TODO -> better error handling -> retry?
		log.Println(err)
		return
	}

	log.Println(response)

	for _, device := range response.DeviceList.Devices {
		if dev, ok := devices[device.DeviceID]; ok {
			dev.host = device.HostName
			dev.deviceType = device.DetectedDiskType
		}
	}

	log.Println(devices)
}

func convertSegmentsToDevices(segments []*segment) (map[uint64]*device, []uint64) {
	result := map[uint64]*device{}
	reqIDs := []uint64{}

	for _, seg := range segments {
		for _, devID := range seg.stripe.deviceIDs {
			if d, ok := result[devID]; ok {
				d.dataSize += uint64(seg.length)
			} else {
				result[devID] = &device{
					id:       devID,
					dataSize: uint64(seg.length),
				}
				reqIDs = append(reqIDs, devID)
			}
		}
	}

	return result, reqIDs
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
// 3.) Allow multiple files and find best fitting node -> works
func (quobyteBackend *QuobyteBackend) GetBestFittingNode(nodes []v1.Node, pod *v1.Pod) (v1.Node, error) {
	defer stop_trace(start_trace())
	var eg errgroup.Group

	log.Printf("Find best fitting Node for Pod: %s\n", pod.ObjectMeta.Name)
	input, err := quobyteBackend.parsePodSpec(pod)
	if err != nil {
		log.Printf("QuobyteBackend: Failed to schedule Pod data-local: %s\n", err)
		return nodes[0], nil
	}

	if quobyteBackend.inKubernetes {
		eg.Go(func() error { return quobyteBackend.getAllDataPods() })
	}

	devices := quobyteBackend.getQuobyteDevices(input)
	if len(devices) == 0 {
		return nodes[0], nil
	}

	if quobyteBackend.inKubernetes {
		quobyteBackend.resolvePodIPToNodeIP(devices)
		if err := eg.Wait(); err != nil {
			log.Printf("QuobyteBackend: Failed to schedule Pod data-local: %s\n", err)
			return nodes[0], nil
		}
	}

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

func (quobyteBackend *QuobyteBackend) getAllDataPods() error {
	log.Println("Get all Data Pod")
	result := map[string]string{}
	podList, err := quobyteBackend.clientset.Core().Pods(quobyteBackend.namespace).List(
		api.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{"role": "data"}),
		})
	if err != nil {
		log.Printf("Error fetching all data nodes %s\n", err)
		return err
	}

	for _, pod := range podList.Items {
		result[pod.Status.PodIP] = pod.Status.HostIP
	}

	quobyteBackend.nodeCache = result
	return nil
}

func (quobyteBackend *QuobyteBackend) resolvePodIPToNodeIP(devices deviceList) {
	for _, device := range devices {
		if hostIP, ok := quobyteBackend.nodeCache[device.host]; ok {
			device.host = hostIP
			log.Printf("Replace Pod IP %s with Host IP %s for Device %d\n", device.host, hostIP, device.id)
		}
	}
}

// Filter all nodes containing no devices
func getDevicesOnPotentialNodes(devices deviceList, nodes []v1.Node) deviceList {
	log.Println("Get potentials nodes")
	res := deviceList{}

	// O(Devices x Nodes) --> better data structure?
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

func validateVolume(volumeName string, volumes []v1.Volume) error {
	for _, vol := range volumes {
		if vol.VolumeSource.Quobyte == nil {
			continue
		}

		if volumeName == vol.VolumeSource.Quobyte.Volume {
			return nil
		}

	}

	return fmt.Errorf("Error: Volume Mount for Volume %s not found in PodSpec\n", volumeName)
}

func (quobyteBackend *QuobyteBackend) parsePodSpec(pod *v1.Pod) (*quobyteInput, error) {
	input := &quobyteInput{files: []string{}}
	var volume string
	var diskType string

	if f, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/files"]; ok {
		split := strings.Split(f, ",")
		input.files = make([]string, len(split))
		for i, file := range split {
			input.files[i] = file
		}
	}

	if d, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/dir"]; ok {
		input.dir = d
	}

	// Files must be in the same volumes -> could be extended for an look up -> what if duplicate
	if v, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/volume"]; ok {
		// If there are more than one Quobyte Volume specified we need some help
		volume = v
		if err := validateVolume(volume, pod.Spec.Volumes); err != nil {
			return input, err
		}
	} else {
		for _, vol := range pod.Spec.Volumes {
			if vol.VolumeSource.Quobyte == nil {
				continue
			}

			volume = vol.VolumeSource.Quobyte.Volume
			break
		}

		if volume == "" {
			return input, fmt.Errorf("Error: No Quobyte Mount found in Podspec for %s", pod.ObjectMeta.Name)
		}
	}

	if d, ok := pod.ObjectMeta.Annotations["scheduler.alpha.quobyte.com.data-aware/type"]; ok {
		// Not implemented
		diskType = d
		_ = diskType
	}

	for i := 0; i < len(input.files); i++ {
		input.files[i] = path.Join(quobyteBackend.quobyteMountpoint, volume, input.files[i])
	}

	if input.dir != "" {
		input.dir = path.Join(quobyteBackend.quobyteMountpoint, volume, input.dir)
	}

	return input, nil
}
