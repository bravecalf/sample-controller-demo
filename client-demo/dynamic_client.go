package k8s_client

import (
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// DynamicClientTest  DynamicClient 动态客户端，可以对任意的K8S资源对象进行操作，包括CRD。
func DynamicClientTest() {
	// config
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		panic(err)
	}

	// client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// get data
	unstructuredList, err := dynamicClient.
		Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).
		Namespace("kube-system").
		List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		panic(err)
	}

	podList := v1.PodList{}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredList.UnstructuredContent(), &podList)
	if err != nil {
		panic(err)
	} else {
		for _, pod := range podList.Items {
			println(pod.Name)
		}
	}
}
