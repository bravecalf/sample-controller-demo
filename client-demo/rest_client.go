package k8s_client

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// RestClientTest RestClient最基础的client，底层是对标准库的net/http的封装，下面的client都是对rest client的封装。
func RestClientTest() {
	// config
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		panic(err)
	}

	config.GroupVersion = &v1.SchemeGroupVersion
	config.NegotiatedSerializer = scheme.Codecs
	config.APIPath = "/api"

	// client
	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		panic(err)
	}

	// get node
	node := v1.Node{}
	err = restClient.Get().Resource("nodes").Name("minikube").Do(context.TODO()).Into(&node)
	if err != nil {
		panic(err)
	} else {
		fmt.Println(node.Name)
	}
}
