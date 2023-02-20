/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	crdexamplecomv1 "sample-controller-demo/pkg/apis/crd.example.com/v1"
	versioned "sample-controller-demo/pkg/generated/clientset/versioned"
	internalinterfaces "sample-controller-demo/pkg/generated/informers/externalversions/internalinterfaces"
	v1 "sample-controller-demo/pkg/generated/listers/crd.example.com/v1"
	time "time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// InferenceInformer provides access to a shared informer and lister for
// Inferences.
type InferenceInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.InferenceLister
}

type inferenceInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewInferenceInformer constructs a new informer for Inference type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewInferenceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredInferenceInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredInferenceInformer constructs a new informer for Inference type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredInferenceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CrdV1().Inferences(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CrdV1().Inferences(namespace).Watch(context.TODO(), options)
			},
		},
		&crdexamplecomv1.Inference{},
		resyncPeriod,
		indexers,
	)
}

func (f *inferenceInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredInferenceInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *inferenceInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&crdexamplecomv1.Inference{}, f.defaultInformer)
}

func (f *inferenceInformer) Lister() v1.InferenceLister {
	return v1.NewInferenceLister(f.Informer().GetIndexer())
}
