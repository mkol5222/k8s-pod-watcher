package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// getClientset returns a Kubernetes clientset.
func getClientset() (*kubernetes.Clientset, error) {
	var kubeconfig *string
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = flag.String("kubeconfig", home+"/.kube/config", "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// In-cluster config or out-of-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
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
	return clientset, nil
}

// watchPodIPChanges watches for changes in pod IPs.
func watchPodIPChanges(clientset *kubernetes.Clientset) {
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
		printPodInfo("+", pod)
	}

	handleUpdate := func(oldObj, newObj interface{}) {
		oldPod := oldObj.(*v1.Pod)
		newPod := newObj.(*v1.Pod)
		if oldPod.Status.PodIP != newPod.Status.PodIP {
			printPodInfo("~", newPod)
		}
	}

	handleDelete := func(obj interface{}) {
		pod := obj.(*v1.Pod)
		printPodInfo("-", pod)
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

// printPodInfo prints the Pod name, namespace, and IP.
func printPodInfo(op string, pod *v1.Pod) {
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
}

func main() {
	clientset, err := getClientset()
	if err != nil {
		fmt.Printf("Error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	watchPodIPChanges(clientset)
}
