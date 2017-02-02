package main

import (
	quobyte_api "github.com/johscheuer/api"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api/v1"
)

type dataLocator struct {
	quobyteClient     *quobyte_api.QuobyteClient
	quobyteMountpoint string
	clientset         *kubernetes.Clientset
}

func newDataLocator(quobyteAPIServer string, quobyteUser string, quobytePassword string, quobyteMountpoint string, clientset *kubernetes.Clientset) *dataLocator {
	return &dataLocator{
		quobyteClient:     quobyte_api.NewQuobyteClient(quobyteAPIServer, quobyteUser, quobytePassword),
		quobyteMountpoint: quobyteMountpoint,
		clientset:         clientset,
	}
}

func (d *dataLocator) find_node(nodes []v1.Node, pod *v1.Pod) (v1.Node, error) {
	// pod.ObjectMeta.
	// TODO get Quobyte metadata -> where is data located
	// parse podSpec for quobyte Mounts if there are non choose random node
	// resolve Pod name to node name (if Quobyte runs containerized)

	// Check MountPoint mit xattr -> http://man7.org/linux/man-pages/man7/xattr.7.html
	// -> per file -> add annotations?

	// getfattr --encoding=text  -n quobyte.info {plugin-dir}/{Volume}/{filename} --> Get devices

	// Ask Quobyte API Where Device are located (on which Node)

	return nodes[0], nil
}

/*

		quobyteClient = quobyte_api.NewQuobyteClient(quobyteAPIServer, quobyteUser, quobytePassword),
		quobyteMountpoint = quobyteMountpoint,
		clientset = clientset,
// wait for merge PR
github.com/johscheuer/api
func main() {
	client := quobyte_api.NewQuobyteClient("http://localhost:7860", "admin", "quobyte")

	endpoints, err := client.GetDeviceNetworkEndpoints(0)
	if err != nil {
		log.Fatalf("Error:", err)
	}

	fmt.Println(endpoints.Endpoints)

	for e := range endpoints.Endpoints {
		fmt.Println(e)
	}
}*/

/*
quobyte.info="posix_attrs {  id: 2  owner: "root"  group: "root"  mode: 33188  atime: 1486028088  ctime: 1486031771  mtime: 1486031771  size: 42949672960  nlinks: 1}system_attrs {  truncate_epoch: 4  issued_truncate_epoch: 4  read_only: false  windows_attributes: 0}storage_layout {  on_disk_format {    block_size_bytes: 4096    object_size_bytes: 8388608    crc_method: CRC_32C  }  distribution {    data_stripe_count: 1    code_stripe_count: 0  }}file_name: "1g.bin"parent_file_id: 1acl {}segment {  start_offset: 0  length: 10737418240  stripe {    version: 4    device_id: 4    device_id: 3    device_id: 10    replica_update_method: QUORUM  }}segment {  start_offset: 10737418240  length: 10737418240  stripe {    version: 1    device_id: 3    device_id: 5    device_id: 10    replica_update_method: QUORUM  }}segment {  start_offset: 21474836480  length: 10737418240  stripe {    version: 1    device_id: 3    device_id: 5    device_id: 4    replica_update_method: QUORUM  }}segment {  start_offset: 32212254720  length: 10737418240  stripe {    version: 1    device_id: 4    device_id: 5    device_id: 10    replica_update_method: QUORUM  }}"
*/
