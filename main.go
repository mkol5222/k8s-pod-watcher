package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// every Pod IP change is a change in the feed
type Change struct {
	Topic string
}

// getClientset returns a Kubernetes clientset.
func getClientset() (*kubernetes.Clientset, error) {
	var kubeconfig *string

	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = flag.String("kubeconfig", home+"/.kube/config", "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
		kubeconfig = &envKubeconfig
	}

	// In-cluster config or out-of-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("kube config: %s\n", *kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	// fmt.Printf("Successfully created Kubernetes clientset %v", clientset)
	return clientset, nil
}

// Action is the action to perform when there are changes for a specific topic
func Action(topic string, count int) {
	fmt.Printf("%s: Action triggered with %d changes\n", topic, count)

	// Prepare the command
	cmd := exec.Command("/bin/bash", "-c", "./refreshFeed.sh "+topic)

	// Run the command and capture the output
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Print the output
	fmt.Println(string(output))
}

// watchPodIPChanges watches for changes in pod IPs.
func watchPodIPChanges(clientset *kubernetes.Clientset) {

	// every pod IP change is a change in the feed
	changes := make(chan Change)
	defer close(changes)

	// Goroutine to monitor changes and perform actions per topic
	go func() {
		const checkIntervalSec = 10
		ticker := time.NewTicker(checkIntervalSec * time.Second)
		defer ticker.Stop()
		changeCount := make(map[string]int)

		for {
			select {
			case change, ok := <-changes:
				if !ok {
					// If channel is closed, perform final actions for all topics with pending changes

					for topic, count := range changeCount {
						if count > 0 {
							Action(topic, count)
						}
					}
					return
				}

				// Update count for the change's topic

				changeCount[change.Topic]++
				fmt.Printf("%s: Received change: %+v\n", change.Topic, change)

			case <-ticker.C:
				// Check counts for each topic and perform actions if there are changes
				for topic, count := range changeCount {
					if count > 0 {
						Action(topic, count)
						// Reset the count after action is performed
						changeCount[topic] = 0
					} else {
						fmt.Printf("%s: No changes in the last %d seconds\n", topic, checkIntervalSec)
					}
				}

			}
		}
	}()

	// Create a ListWatch for Pods
	listWatch := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"pods",
		v1.NamespaceAll,
		fields.Everything(),
	)

	// Define event handler functions
	handleAdd := func(obj interface{}) {
		pod := obj.(*v1.Pod)
		handlePodChange("+", pod, changes)
	}

	handleUpdate := func(oldObj, newObj interface{}) {
		oldPod := oldObj.(*v1.Pod)
		newPod := newObj.(*v1.Pod)
		if oldPod.Status.PodIP != newPod.Status.PodIP {
			handlePodChange("~", newPod, changes)
		}
	}

	handleDelete := func(obj interface{}) {
		pod := obj.(*v1.Pod)
		handlePodChange("-", pod, changes)
	}

	// Create an Informer
	informer := cache.NewSharedInformer(
		listWatch,
		&v1.Pod{},
		0, // Resync period, 0 to disable resync
	)

	// Set up event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handleAdd,
		UpdateFunc: handleUpdate,
		DeleteFunc: handleDelete,
	})

	stopCh := make(chan struct{})
	defer close(stopCh)

	// Signal handler to gracefully shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalCh
		fmt.Println("Shutting down watcher...")
		close(stopCh)
	}()

	fmt.Println("Starting pod watcher...")
	informer.Run(stopCh)
}

// handlePodChange prints the Pod name, namespace, and IP.
func handlePodChange(op string, pod *v1.Pod, changes chan Change) {
	ip := "<none>"
	if pod.Status.PodIP != "" {
		ip = pod.Status.PodIP
	}

	// Collect labels
	labels := "None"
	if len(pod.Labels) > 0 {
		labels = ""
		for key, value := range pod.Labels {
			labels += fmt.Sprintf("%s=%s, ", key, value)
		}
		// Trim the trailing comma and space
		labels = labels[:len(labels)-2]
	}

	fmt.Printf("%s: IP: %s Pod: %s, Namespace: %s, Labels: %s \n", op, ip, pod.Name, pod.Namespace, labels)
	if pod.Status.PodIP != "" {
		reportPodIpUpdate(pod, changes)
	}
}

// reportPodIpUpdate counts the pod IP updates
func reportPodIpUpdate(pod *v1.Pod, changes chan Change) {

	// extract app label
	appLabel := pod.Labels["app"]
	if appLabel != "" {
		//fmt.Printf("App label: %s\n", appLabel)
		// combine namespace and app label to uniq key
		key := fmt.Sprintf("%s-%s", pod.Namespace, appLabel)
		// fmt.Printf("Key: %s\n", key)

		changes <- Change{Topic: key}
	}

	// fmt.Printf("Reporting IP update for pod %s in namespace %s with IP %s\n", pod.Name, pod.Namespace, pod.Status.PodIP)
}

func main() {

	clientset, err := getClientset()
	if err != nil {
		fmt.Printf("Error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	watchPodIPChanges(clientset)
}
