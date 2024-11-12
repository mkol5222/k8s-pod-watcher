package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type PodInfo struct {
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
}

type Request struct {
	Label     string `json:"label"`
	Namespace string `json:"namespace"`
}

func main() {
	// Kubernetes client setup
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		log.Fatalf("Error loading kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	// HTTP server setup
	http.HandleFunc("/pods", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Get label and namespace from query parameters
			label := r.URL.Query().Get("label")
			namespace := r.URL.Query().Get("namespace")

			// Check if both label and namespace are provided
			if label == "" || namespace == "" {
				http.Error(w, "Missing 'label' or 'namespace' query parameter", http.StatusBadRequest)
				return
			}

			// Query Kubernetes for pods in the specified namespace with the specified label
			pods, err := getPodsByLabel(clientset, namespace, label)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error retrieving pods: %v", err), http.StatusInternalServerError)
				return
			}

			// Return the list of pod names and IPs as a JSON response
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(pods); err != nil {
				http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
			}
		} else if r.Method == http.MethodPost {
			var req Request
			// Decode the request body into a Request struct
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, fmt.Sprintf("Invalid input: %v", err), http.StatusBadRequest)
				return
			}
			//
			fmt.Printf("Request: %+v\n", req)

			// Query Kubernetes for pods in the specified namespace with the specified label
			pods, err := getPodsByLabel(clientset, req.Namespace, req.Label)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error retrieving pods: %v", err), http.StatusInternalServerError)
				return
			}

			// Return the list of pod names and IPs as a JSON response
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(pods); err != nil {
				http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
			}
		} else {
			http.Error(w, "Invalid HTTP method", http.StatusMethodNotAllowed)
		}
	})

	log.Println("Starting HTTP server on port 9090...")
	log.Fatal(http.ListenAndServe(":9090", nil))
}

// getPodsByLabel queries Kubernetes for pods based on the given namespace and label
func getPodsByLabel(clientset *kubernetes.Clientset, namespace, labelSelector string) ([]PodInfo, error) {
	// Define the label selector to use in the pod query
	labelSelector = strings.TrimSpace(labelSelector)
	if labelSelector == "" {
		return nil, fmt.Errorf("label selector cannot be empty")
	}

	// Get the list of pods in the specified namespace
	podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("error querying pods: %v", err)
	}

	var pods []PodInfo
	for _, pod := range podList.Items {
		pods = append(pods, PodInfo{
			Name:      pod.Name,
			IPAddress: pod.Status.PodIP,
		})
	}

	return pods, nil
}
