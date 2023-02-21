package main

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	mycontroller "sample-controller-demo/controller"
	"sample-controller-demo/pkg/generated/clientset/versioned"
	"sample-controller-demo/pkg/generated/informers/externalversions"
)

func main() {
	// 测试自定义crd的获取调用
	//k8s_client.CrdDemoTest()
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		log.Fatalf("Failed to get k8s config from flags, error: %v \n", err)
		return
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to get clientset with config %#v, error: %v \n", config, err)
		return
	}

	crdClient, err := versioned.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to get crdClient with config %#v, error: %v \n", config, err)
		return
	}

	listOptions := func(options *metav1.ListOptions) {

	}

	crdInformerFactory := externalversions.NewSharedInformerFactory(crdClient, 0)

	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientSet, 0, informers.WithNamespace("default"), informers.WithTweakListOptions(listOptions))
	controller := mycontroller.NewInferenceController(
		crdInformerFactory.Crd().V1().Inferences(),
		informerFactory.Apps().V1().Deployments(),
		informerFactory.Core().V1().Services(),
		informerFactory.Networking().V1().Ingresses(),
		clientSet,
		crdClient)

	stopChan := make(chan struct{})
	informerFactory.Start(stopChan)
	crdInformerFactory.Start(stopChan)

	controller.Run(context.Background(), 5)
}
