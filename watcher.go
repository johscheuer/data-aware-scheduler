package main

import (
	"fmt"
	"log"
	"strings"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/unversioned"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/fields"
	"k8s.io/client-go/1.5/pkg/watch"
)

func getNodes(clientset *kubernetes.Clientset) (*v1.NodeList, error) {
	nodes, err := clientset.Core().Nodes().List(api.ListOptions{})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

func watchUnscheduledPods(clientset *kubernetes.Clientset) (<-chan *v1.Pod, <-chan error) {
	pods := make(chan *v1.Pod)
	errc := make(chan error, 1)

	podWatch, err := clientset.Core().Pods(api.NamespaceAll).Watch(
		api.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("spec.nodeName", ""),
		})

	if err != nil {
		errc <- err
	}

	go func() {
		select {
		case event, ok := <-podWatch.ResultChan():
			if !ok {
				// break the loop
			}

			if event.Type == watch.Error {
				errc <- fmt.Errorf("Error occured")
				//errc <- event.Object.(*api.Status)
			}

			if event.Type == watch.Added {
				unscheduled_pod := event.Object.(*v1.Pod)

				if data_aware_annotated(unscheduled_pod) {
					pods <- unscheduled_pod
				}
			}
		}
	}()

	return pods, errc
}

// Move this into FieldSelector ?
func data_aware_annotated(pod *v1.Pod) bool {
	if val, ok := pod.ObjectMeta.Annotations["scheduler.alpha.kubernetes.io/name"]; ok {
		return val == schedulerName
	}

	// no annotation
	return false
}

func getUnscheduledPods(clientset *kubernetes.Clientset) ([]*v1.Pod, error) {
	unscheduledPods := []*v1.Pod{}
	podList, err := clientset.Core().Pods(api.NamespaceAll).List(
		api.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("spec.nodeName", ""),
		})
	if err != nil {
		return unscheduledPods, err
	}

	for _, pod := range podList.Items {
		if data_aware_annotated(&pod) {
			unscheduledPods = append(unscheduledPods, &pod)
		}
	}

	return unscheduledPods, nil
}

func getAllocatableNodes(nodes []v1.Node, requiredResources map[string]int64) ([]v1.Node, []string) {
	var allocatableNodes []v1.Node
	fitFailures := make([]string, 0)

	for _, node := range nodes {
		if allocatableCpu, ok := node.Status.Allocatable["cpu"]; ok {
			log.Printf("CPU: Alloc: %v - Needed: %v", allocatableCpu.MilliValue(), requiredResources["cpu"])
			if allocatableCpu.MilliValue() < requiredResources["cpu"] {
				fitFailures = append(fitFailures, fmt.Sprintf("fit failure on node (%s): Requested %v CPU only %v CPU free", node.ObjectMeta.Name, allocatableCpu.MilliValue(), requiredResources["cpu"]))
				continue
			}
		}

		if allocatableMemory, ok := node.Status.Allocatable["memory"]; ok {
			log.Printf("Memory: Alloc: %v - Needed: %v", allocatableMemory.Value(), requiredResources["memory"])
			if allocatableMemory.Value() < requiredResources["memory"] {
				fitFailures = append(fitFailures, fmt.Sprintf("fit failure on node (%s): Requested %v memory only %v memory free", node.ObjectMeta.Name, allocatableMemory.Value(), requiredResources["memory"]))
				continue
			}
		}

		allocatableNodes = append(nodes, node)
	}

	return allocatableNodes, fitFailures
}

func getRequiredResources(pod *v1.Pod) map[string]int64 {
	requiredResources := map[string]int64{
		"cpu":    0,
		"memory": 0}

	for _, container := range pod.Spec.Containers {
		for rName, rQuantity := range container.Resources.Requests {
			switch rName {
			case v1.ResourceMemory:
				requiredResources["memory"] += rQuantity.Value()
			case v1.ResourceCPU:
				requiredResources["cpu"] += rQuantity.MilliValue()
			default:
				log.Println("Unkown Resource")
			}
		}
	}

	return requiredResources
}

func fit(clientset *kubernetes.Clientset, pod *v1.Pod) ([]v1.Node, error) {
	nodeList, err := getNodes(clientset)
	if err != nil {
		return []v1.Node{}, err
	}

	nodes, fitFailures := getAllocatableNodes(nodeList.Items, getRequiredResources(pod))

	if len(nodes) == 0 {
		err := sendEvent(
			clientset,
			fmt.Sprintf("pod (%s) failed to fit in any node\n%s", pod.ObjectMeta.Name, strings.Join(fitFailures, "\n")),
			"FailedScheduling",
			"Warning",
			pod)

		if err != nil {
			return []v1.Node{}, err
		}
	}

	return nodes, nil
}

func bind(clientset *kubernetes.Clientset, pod *v1.Pod, node v1.Node) error {
	binding := &v1.Binding{
		TypeMeta: unversioned.TypeMeta{
			APIVersion: pod.TypeMeta.APIVersion,
			Kind:       "Binding",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      pod.ObjectMeta.Name,
			Namespace: pod.ObjectMeta.Namespace,
		},
		Target: v1.ObjectReference{
			APIVersion: pod.TypeMeta.APIVersion,
			Kind:       node.TypeMeta.Kind,
			Name:       node.ObjectMeta.Name,
			Namespace:  node.ObjectMeta.Namespace,
		},
	}

	if err := clientset.Core().Pods(pod.ObjectMeta.Namespace).Bind(binding); err != nil {
		return fmt.Errorf("Error in: %s - %v", "bind", err)
	}

	// Emit a Kubernetes event that the Pod was scheduled successfully.
	return sendEvent(
		clientset,
		fmt.Sprintf("Successfully assigned %s to %s", pod.ObjectMeta.Name, node.ObjectMeta.Name),
		"Scheduled",
		"Normal",
		pod)
}

func sendEvent(clientset *kubernetes.Clientset, msg string, reason string, eventType string, pod *v1.Pod) error {
	event := &v1.Event{
		Count:   1,
		Message: msg,
		ObjectMeta: v1.ObjectMeta{
			GenerateName: pod.ObjectMeta.Name + "-",
			Namespace:    pod.ObjectMeta.Namespace,
		},
		Reason:         reason,
		LastTimestamp:  unversioned.Now(),
		FirstTimestamp: unversioned.Now(),
		Type:           eventType,
		Source: v1.EventSource{
			Component: schedulerName,
		},
		InvolvedObject: v1.ObjectReference{
			Kind:      pod.TypeMeta.Kind,
			Name:      pod.ObjectMeta.Name,
			Namespace: pod.ObjectMeta.Namespace,
			UID:       pod.ObjectMeta.UID,
		},
	}

	_, err := clientset.Core().Events(pod.ObjectMeta.Namespace).Create(event)

	return err
}
