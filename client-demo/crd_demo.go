package k8s_client

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"sample-controller-demo/pkg/generated/clientset/versioned"
)

func CrdDemoTest() {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		log.Fatalln(err)
	}

	client, err := versioned.NewForConfig(config)
	if err != nil {
		log.Fatalln(err)
	}

	inference, err := client.CrdV1().Inferences("default").Get(context.TODO(), "my-inference", metav1.GetOptions{})
	if err != nil {
		log.Fatalln(err)
	} else {
		fmt.Printf("get inference is %#v", inference)
	}
}
