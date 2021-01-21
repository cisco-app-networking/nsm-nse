package multi_ns_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	clientset *kubernetes.Clientset
	ip_addr   = "127.0.0.1"
	clientDep = &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-client",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"networkservicemesh.io/app": "simple-client",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"networkservicemesh.io/app": "simple-client",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "alpine-img",
							Image:           "alpine:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"tail", "-f", "/dev/null"},
						},
					},
				},
			},
		},
	}

	// dep1 = &appsv1.Deployment{
	// 	TypeMeta: metav1.TypeMeta{
	// 		APIVersion: "apps/v1",
	// 	},
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      "bridge-domain",
	// 		Namespace: "default",
	// 	},
	// 	Spec: appsv1.DeploymentSpec{
	// 		Selector: &metav1.LabelSelector{
	// 			MatchLabels: map[string]string{
	// 				"networkservicemesh.io/app":  "bridge-domain",
	// 				"networkservicemesh.io/impl": "bridge",
	// 			},
	// 		},
	// 		Template: corev1.PodTemplateSpec{
	// 			ObjectMeta: metav1.ObjectMeta{
	// 				Labels: map[string]string{
	// 					"networkservicemesh.io/app":  "bridge-domain",
	// 					"networkservicemesh.io/impl": "bridge",
	// 				},
	// 			},
	// 			Spec: corev1.PodSpec{
	// 				Containers: []corev1.Container{
	// 					{
	// 						Name:  "bridge-domain",
	// 						Image: "networkservicemesh/bridge-domain-bridge:master",
	// 						Env: []corev1.EnvVar{
	// 							{
	// 								Name:  "ENDPOINT_NETWORK_SERVICE",
	// 								Value: "bridge-domain",
	// 							},
	// 							{
	// 								Name:  "ENDPOINT_LABELS",
	// 								Value: "app=bridge",
	// 							},
	// 							{
	// 								Name:  "TRACER_ENABLED",
	// 								Value: "true",
	// 							},
	// 							{
	// 								Name:  "IP_ADDRESS",
	// 								Value: "10.60.1.0/24",
	// 							},
	// 						},
	// 						Resources: corev1.ResourceRequirements{
	// 							Limits: map[corev1.ResourceName]resource.Quantity{
	// 								"networkservicemesh.io/socket": resource.MustParse("1"),
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	// dep2 = &appsv1.Deployment{
	// 	TypeMeta: metav1.TypeMeta{
	// 		APIVersion: "apps/v1",
	// 	},
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      "bridge-domain-ipv6",
	// 		Namespace: "default",
	// 	},
	// 	Spec: appsv1.DeploymentSpec{
	// 		Selector: &metav1.LabelSelector{
	// 			MatchLabels: map[string]string{
	// 				"networkservicemesh.io/app":  "bridge-domain-ipv6",
	// 				"networkservicemesh.io/impl": "bridge-ipv6",
	// 			},
	// 		},
	// 		Template: corev1.PodTemplateSpec{
	// 			ObjectMeta: metav1.ObjectMeta{
	// 				Labels: map[string]string{
	// 					"networkservicemesh.io/app":  "bridge-domain-ipv6",
	// 					"networkservicemesh.io/impl": "bridge-ipv6",
	// 				},
	// 			},
	// 			Spec: corev1.PodSpec{
	// 				Containers: []corev1.Container{
	// 					{
	// 						Name:  "bridge-domain",
	// 						Image: "networkservicemesh/bridge-domain-bridge:master",
	// 						Env: []corev1.EnvVar{
	// 							{
	// 								Name:  "ENDPOINT_NETWORK_SERVICE",
	// 								Value: "bridge-domain-ipv6",
	// 							},
	// 							{
	// 								Name:  "ENDPOINT_LABELS",
	// 								Value: "app=bridge-ipv6",
	// 							},
	// 							{
	// 								Name:  "TRACER_ENABLED",
	// 								Value: "true",
	// 							},
	// 							{
	// 								Name:  "IP_ADDRESS",
	// 								Value: "1200::/120",
	// 							},
	// 						},
	// 						Resources: corev1.ResourceRequirements{
	// 							Limits: map[corev1.ResourceName]resource.Quantity{
	// 								"networkservicemesh.io/socket": resource.MustParse("1"),
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }
)

func TestMain(m *testing.M) {
	const clusterName string = "test"
	//Remove existing cluster which has the same name
	fmt.Println("Remove old cluster")
	removeExistingKindCluster(clusterName)
	//Create a kind cluster for testing
	fmt.Println("Creating new kind cluster...")
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

	os.Exit(code)
}
func TestSingleClientMultiNS(t *testing.T) {
	for testName, c := range map[string]struct {
		networkServices []string
		annotations     map[string]string
	}{
		"Connect to one NSE": {
			[]string{"foo"},
			map[string]string{"ns.networkservicemesh.io": "foo?app=vl3-nse-foo"},
		},
		"Connect to two NSEs": {
			[]string{"foo", "bar"},
			map[string]string{"ns.networkservicemesh.io": "foo?app=vl3-nse-foo,bar?app=vl3-nse-bar"},
		},
	} {
		t.Logf("Running test case: %s", testName)
		var deployments []appsv1.Deployment

		//Install network service
		deploymentsClient := clientset.AppsV1().Deployments(corev1.NamespaceDefault)
		for _, name := range c.networkServices {
			fmt.Printf("Installing Network Service %s\n", name)
			os.Setenv("REMOTE_IP", ip_addr)
			os.Setenv("SERVICENAME", name)
			cmd := exec.Command("../../scripts/vl3/vl3_interdomain.sh")
			err := cmd.Run()
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}
		endpoints := listDeployments("wcm-system", clientset)
		deployments = append(deployments, endpoints...)

		//Create client pod with annotation
		clientDep.ObjectMeta.Annotations = c.annotations
		fmt.Println("Create client deployment")
		result, err := deploymentsClient.Create(context.TODO(), clientDep, metav1.CreateOptions{})
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		deployments = append(deployments, *result)

		var ready bool
		for i := 0; i < 12; i++ {
			fmt.Println("Waiting for client pod to start...")
			time.Sleep(time.Duration(5) * time.Second)
			ready = checkAvailability(result, clientset)
			if ready {
				break
			}
		}
		if !ready {
			t.Error("Client pod NOT connected to networkservices!")
		} else {
			t.Log(("Success! Client pod connected to networkservices!"))
		}

		//Cleanup deployments
		for _, d := range deployments {
			removeDeployment(d.ObjectMeta.Namespace, d.ObjectMeta.Name, clientset)
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

func removeDeployment(namespace, deploymentName string, clientset *kubernetes.Clientset) {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	if err := deploymentsClient.Delete(context.TODO(), deploymentName, metav1.DeleteOptions{}); err != nil {
		fmt.Println(err.Error())
	}
	fmt.Printf("Deleted deployment %s\n", deploymentName)
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

func checkAvailability(deployment *appsv1.Deployment, clientset *kubernetes.Clientset) bool {
	namespace := deployment.ObjectMeta.Namespace
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Println(err.Error())
	}

	for _, pod := range pods.Items {
		fmt.Println(pod.Status.Phase)
		if pod.Status.Phase != corev1.PodRunning {
			return false
		}
	}
	return true
}

func listDeployments(namespace string, clientset *kubernetes.Clientset) []appsv1.Deployment {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	list, err := deploymentsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Println(err.Error())
	}
	return list.Items
}

func int32Ptr(i int32) *int32 { return &i }
