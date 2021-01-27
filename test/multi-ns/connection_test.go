package multi_ns_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	clientset *kubernetes.Clientset
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
)

func TestSingleClientMultiNS(t *testing.T) {
	for testName, c := range map[string]struct {
		networkServices []string
		annotations     map[string]string
	}{
		"Connect to one NSE": {
			[]string{"foo"},
			map[string]string{"ns.networkservicemesh.io": "foo?app=vl3-nse-foo"},
		},
		// "Connect to two NSEs": {
		// 	[]string{"foo", "bar"},
		// 	map[string]string{"ns.networkservicemesh.io": "foo?app=vl3-nse-foo,bar?app=vl3-nse-bar"},
		// },
	} {
		t.Logf("Running test case: %s", testName)
		var deployments []appsv1.Deployment

		//Install network service
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
		deploymentsClient := clientset.AppsV1().Deployments(corev1.NamespaceDefault)
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

func removeDeployment(namespace, deploymentName string, clientset *kubernetes.Clientset) {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	if err := deploymentsClient.Delete(context.TODO(), deploymentName, metav1.DeleteOptions{}); err != nil {
		fmt.Println(err.Error())
	}
	fmt.Printf("Deleted deployment %s\n", deploymentName)
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
