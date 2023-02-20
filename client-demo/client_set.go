package k8s_client

import (
	"context"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientSetTest 基于Rest Client进行了封装，创建出各个GVR的client, 通过clientset可以更加方便地操作K8S地资源对象。
func ClientSetTest() {
	// config
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		panic(err)
	}

	// clientSet
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// get data
	podList, err := clientSet.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{})
	if err != nil {
		panic(err)
	} else {
		for _, pod := range podList.Items {
			println(pod.Name)
		}
	}

}
