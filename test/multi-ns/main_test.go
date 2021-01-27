package multi_ns_test

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func TestMain(m *testing.M) {
	var clusterName string
	flag.StringVar(&clusterName, "clusterName", "test", "Name of kind cluster for this test")
	flag.Parse()
	//Remove existing cluster which has the same name
	fmt.Println("Remove old cluster")
	removeExistingKindCluster(clusterName)
	//Create a kind cluster for testing
	fmt.Printf("Creating kind cluster '%s'\n", clusterName)
	execKindCluster("create", clusterName)
	//Prepare clientset for K8s API
	clientset, _ = getClientSet()

	//Install NSM
	fmt.Println("Installing NSM...")
	cmd := exec.Command("../../scripts/vl3/nsm_install_interdomain.sh")
	err := cmd.Run()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	code := m.Run()
	//Remove the cluster after tests are done
	execKindCluster("delete", clusterName)
	fmt.Printf("Deleted kind cluster '%s'\n", clusterName)
	os.Exit(code)
}

func removeExistingKindCluster(clusterName string) {
	//Get all kind clusters
	out, err := exec.Command("kind", "get", "clusters").Output()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	//Check if input cluster already exists
	//If found, remove it
	s := strings.Fields(string(out))
	for _, name := range s {
		if name == clusterName {
			execKindCluster("delete", clusterName)
		}
	}
}

func execKindCluster(action, clusterName string) {
	nameFlag := "--name=" + clusterName
	cmd := exec.Command("kind", action, "cluster", nameFlag)
	err := cmd.Run()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
}

func getClientSet() (*kubernetes.Clientset, error) {
	//Get kubeconfig
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	return clientset, nil
}
