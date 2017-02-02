package main

import (
	"fmt"
	"strings"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/resource"
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
				pods <- event.Object.(*v1.Pod)
			}
		}
	}()

	return pods, errc
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
		if pod.ObjectMeta.Annotations["scheduler.alpha.kubernetes.io/name"] == schedulerName {
			unscheduledPods = append(unscheduledPods, &pod)
		}
	}

	return unscheduledPods, nil
}

func getPods(clientset *kubernetes.Clientset) (*v1.PodList, error) {
	pods, err := clientset.Core().Pods(api.NamespaceAll).List(api.ListOptions{})
	if err != nil {
		return nil, err
	}

	return pods, nil
}

func fit(clientset *kubernetes.Clientset, pod *v1.Pod) ([]v1.Node, error) {
	nodeList, err := getNodes(clientset)
	if err != nil {
		return []v1.Node{}, err
	}

	podList, err := getPods(clientset)
	if err != nil {
		return []v1.Node{}, err
	}

	//TODO check memory too
	resourceUsage := make(map[string]*resource.Quantity)
	for _, node := range nodeList.Items {
		resourceUsage[node.ObjectMeta.Name] = &resource.Quantity{}
	}

	for _, p := range podList.Items {
		if p.Spec.NodeName == "" {
			continue
		}
		for _, c := range p.Spec.Containers {
			if cpu, ok := c.Resources.Requests["cpu"]; ok {
				resourceUsage[p.Spec.NodeName].Add(cpu)
			}
		}
	}

	var nodes []v1.Node
	fitFailures := make([]string, 0)

	var spaceRequired resource.Quantity
	for _, c := range pod.Spec.Containers {
		if cpu, ok := c.Resources.Requests["cpu"]; ok {
			spaceRequired.Add(cpu)
		}
	}

	for _, node := range nodeList.Items {
		if cpu, ok := node.Status.Allocatable["cpu"]; ok {
			cpu.Sub(*resourceUsage[node.ObjectMeta.Name])
			if cpu.Cmp(spaceRequired) <= 0 {
				fitFailures = append(fitFailures, fmt.Sprintf("fit failure on node (%s): Insufficient CPU", node.ObjectMeta.Name))
				continue
			}
			nodes = append(nodes, node)
		}
	}

	if len(nodes) == 0 {
		event := &v1.Event{
			Count:   1,
			Message: fmt.Sprintf("pod (%s) failed to fit in any node\n%s", pod.ObjectMeta.Name, strings.Join(fitFailures, "\n")),
			ObjectMeta: v1.ObjectMeta{
				GenerateName: pod.ObjectMeta.Name + "-",
				Namespace:    pod.ObjectMeta.Namespace,
			},
			Reason:         "FailedScheduling",
			LastTimestamp:  unversioned.Now(),
			FirstTimestamp: unversioned.Now(),
			Type:           "Warning",
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
	event := &v1.Event{
		Count:   1,
		Message: fmt.Sprintf("Successfully assigned %s to %s", pod.ObjectMeta.Name, node.ObjectMeta.Name),
		ObjectMeta: v1.ObjectMeta{
			GenerateName: pod.ObjectMeta.Name + "-",
			Namespace:    pod.ObjectMeta.Namespace,
		},
		Reason:         "Scheduled",
		LastTimestamp:  unversioned.Now(),
		FirstTimestamp: unversioned.Now(),
		Type:           "Normal",
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
