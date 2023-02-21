package controller

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	networkinginformers "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	networkinglisters "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"log"
	"reflect"
	crdv1 "sample-controller-demo/pkg/apis/crd.example.com/v1"
	"sample-controller-demo/pkg/generated/clientset/versioned"
	crdinformers "sample-controller-demo/pkg/generated/informers/externalversions/crd.example.com/v1"
	crdlisters "sample-controller-demo/pkg/generated/listers/crd.example.com/v1"
	"time"
)

const maxRetries int = 15

var inferenceKind = crdv1.SchemeGroupVersion.WithKind("Inference")

type InferenceController struct {
	// k8s标准 clientset
	kubeclient kubernetes.Interface
	// sample 生成的 clientset
	crdclient versioned.Interface

	ifLister crdlisters.InferenceLister
	dLister  appslisters.DeploymentLister
	sLister  corelisters.ServiceLister
	igLister networkinglisters.IngressLister

	ifListerSynced   cache.InformerSynced
	dListerSynced    cache.InformerSynced
	sListerSynced    cache.InformerSynced
	igListerSynced   cache.InformerSynced
	enqueueInference func(*crdv1.Inference)

	syncHandler func(context.Context, string) error

	queue workqueue.RateLimitingInterface
}

func NewInferenceController(
	ifInformer crdinformers.InferenceInformer,
	dInformer appsinformers.DeploymentInformer,
	sInformer coreinformers.ServiceInformer,
	igInformer networkinginformers.IngressInformer,
	kubeclient kubernetes.Interface,
	crdclient versioned.Interface) *InferenceController {

	ic := &InferenceController{
		kubeclient: kubeclient,
		crdclient:  crdclient,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "inference"),
	}

	ifInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ic.addInference,
		UpdateFunc: ic.updateInference,
	})

	dInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ic.addDeployment,
		UpdateFunc: ic.updateDeployment,
		DeleteFunc: ic.deleteDeployment,
	})

	sInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ic.handleObject,
		UpdateFunc: ic.updateService,
		DeleteFunc: ic.handleObject,
	})

	igInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ic.handleObject,
		UpdateFunc: ic.updateIngress,
		DeleteFunc: ic.handleObject,
	})

	ic.enqueueInference = ic.enqueue
	ic.syncHandler = ic.syncInference
	ic.dLister = dInformer.Lister()
	ic.sLister = sInformer.Lister()
	ic.igLister = igInformer.Lister()
	ic.ifLister = ifInformer.Lister()

	ic.dListerSynced = dInformer.Informer().HasSynced
	ic.sListerSynced = sInformer.Informer().HasSynced
	ic.igListerSynced = igInformer.Informer().HasSynced
	ic.ifListerSynced = ifInformer.Informer().HasSynced

	return ic
}

func (ic *InferenceController) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer ic.queue.ShutDown()

	log.Println("Starting inference controller...")
	defer log.Println("Shutting down inference controller...")

	// 等待第一次从kube-apiserver中获取到的全量的对象是否全部从DeltaFIFO中pop完成，存入到indexer缓存中。
	if !cache.WaitForNamedCacheSync("inference", ctx.Done(), ic.dListerSynced, ic.sListerSynced, ic.igListerSynced, ic.ifListerSynced) {
		return
	}

	log.Println("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, ic.worker, time.Second)
	}

	log.Println("Started workers")
	<-ctx.Done()
	log.Println("Shutting down workers")
}

func (ic *InferenceController) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) *crdv1.Inference {
	if controllerRef.Kind != inferenceKind.Kind {
		return nil
	}

	infer, err := ic.ifLister.Inferences(namespace).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if infer.UID != controllerRef.UID {
		return nil
	}
	return infer
}

func (ic *InferenceController) addDeployment(obj interface{}) {
	deploy := obj.(*appsv1.Deployment)
	if ownerRef := metav1.GetControllerOf(deploy); ownerRef != nil {
		infer := ic.resolveControllerRef(deploy.Namespace, ownerRef)
		if infer == nil {
			return
		}
		log.Printf("Adding deployment [%s] in namespace [%s]. \n", deploy.Name, deploy.Namespace)
		ic.enqueue(infer)
	}
}

func (ic *InferenceController) updateDeployment(oldObj interface{}, newObj interface{}) {
	oldD := oldObj.(*appsv1.Deployment)
	newD := newObj.(*appsv1.Deployment)
	// 当deploy版本变更，或者可用副本数变更时，入队进行处理，变更inference status
	if oldD.ResourceVersion != newD.ResourceVersion || oldD.Status.AvailableReplicas != newD.Status.AvailableReplicas {
		if ownerRef := metav1.GetControllerOf(newD); ownerRef != nil {
			infer := ic.resolveControllerRef(newD.Namespace, ownerRef)
			if infer == nil {
				return
			}
			log.Printf("Updating deployment [%s] in namespace [%s] from rv-replicas[%s-%d] to rv-replicas[%s-%d]. \n",
				newD.Name, newD.Namespace, oldD.ResourceVersion, oldD.Status.AvailableReplicas, newD.ResourceVersion, newD.Status.AvailableReplicas)
			ic.enqueue(infer)
		}
	}
}

func (ic *InferenceController) deleteDeployment(obj interface{}) {
	deploy, ok := obj.(*appsv1.Deployment)
	if !ok {
		// 将stone中的obj resync 同步到deltaFIFO中
		tombstone, ok := obj.(*cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		} else {
			deploy, ok = tombstone.Obj.(*appsv1.Deployment)
			if !ok {
				runtime.HandleError(fmt.Errorf("tombstone contained object that is not a Deployment %#v", obj))
				return
			}
		}
	}

	if ownerRef := metav1.GetControllerOf(deploy); ownerRef != nil {
		infer := ic.resolveControllerRef(deploy.Namespace, ownerRef)
		if infer == nil {
			return
		}
		log.Printf("Deleting deployment [%s] in namespace [%s]. \n", deploy.Name, deploy.Namespace)
		ic.enqueue(infer)
	}
}

func (ic *InferenceController) updateService(oldObj interface{}, newObj interface{}) {
	oldS := oldObj.(*corev1.Service)
	newS := newObj.(*corev1.Service)

	if reflect.DeepEqual(oldS, newS) {
		return
	}

	if controllerRef := metav1.GetControllerOf(newS); controllerRef != nil {
		infer := ic.resolveControllerRef(newS.Namespace, controllerRef)
		if infer == nil {
			return
		}
		log.Printf("Updating service [%s] in namespace [%s]. \n", infer.Name, infer.Namespace)
		ic.enqueue(infer)
	}
}

func (ic *InferenceController) updateIngress(oldObj interface{}, newObj interface{}) {
	oldIg := oldObj.(*networkingv1.Ingress)
	newIg := newObj.(*networkingv1.Ingress)

	if reflect.DeepEqual(oldIg, newIg) {
		return
	}

	if controllerRef := metav1.GetControllerOf(newIg); controllerRef != nil {
		infer := ic.resolveControllerRef(newIg.Namespace, controllerRef)
		if infer == nil {
			return
		}
		log.Printf("Updating ingress [%s] in namespace [%s]. \n", infer.Name, infer.Namespace)
		ic.enqueue(infer)
	}
}

func (ic *InferenceController) enqueue(inference *crdv1.Inference) {
	// 获取key {namespace}-{name}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(inference)
	if err != nil {
		runtime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", inference, err))
		return
	}
	ic.queue.Add(key)
}

func (ic *InferenceController) worker(ctx context.Context) {
	for ic.processNextWorkItem(ctx) {

	}
}

func (ic *InferenceController) processNextWorkItem(ctx context.Context) bool {
	key, shutdown := ic.queue.Get()
	if shutdown {
		return false
	}
	defer ic.queue.Done(key)

	err := ic.syncHandler(ctx, key.(string))
	ic.syncError(err, key)

	return true
}

func (ic *InferenceController) syncInference(ctx context.Context, dKey string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(dKey)
	if err != nil {
		log.Printf("Failed to split meta namespace cache key, err: %v", err)
		return err
	}

	infer, err := ic.ifLister.Inferences(namespace).Get(name)
	if errors.IsNotFound(err) {
		// 判断infer被删除, 直接删除相关的deployment、service、ingress
		//if err := ic.kubeclient.AppsV1().Deployments(namespace).Delete(ctx, infer.Spec.Deployment.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		//	log.Printf("Failed to delete deployment name[%s] in namespace[%s], error: %v \n", infer.Spec.Deployment.Name, namespace, err)
		//	return err
		//}
		//
		//if err := ic.kubeclient.CoreV1().Services(namespace).Delete(ctx, infer.Spec.Service.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		//	log.Printf("Failed to delete service name[%s] in namespace[%s], error: %v \n", infer.Spec.Deployment.Name, namespace, err)
		//	return err
		//}
		//
		//if err := ic.kubeclient.NetworkingV1().Ingresses(namespace).Delete(ctx, infer.Spec.Ingress.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		//	log.Printf("Failed to delete ingress name[%s] in namespace[%s], error: %v \n", infer.Spec.Ingress.Name, namespace, err)
		//	return err
		//}
		log.Println("HHH TEST")
	}

	if err != nil {
		return err
	}

	// 同步deployment可用副本数
	deploy, err := ic.dLister.Deployments(namespace).Get(infer.Spec.Deployment.Name)
	if errors.IsNotFound(err) {
		// 如果deployment不存在，则创建deployment
		newD := ic.constructDeployment(infer)
		_, errD := ic.kubeclient.AppsV1().Deployments(namespace).Create(ctx, newD, metav1.CreateOptions{})
		if errD != nil {
			log.Printf("Failed to create deployment [%#v], error: %v\n", deploy, errD)
			return errD
		}
	} else if err != nil {
		return err
	} else {
		// deployment 副本数配置与inference副本数相同
		if *deploy.Spec.Replicas != infer.Spec.Deployment.Replicas {
			_, errD := ic.kubeclient.AppsV1().Deployments(namespace).Update(ctx, ic.constructDeployment(infer), metav1.UpdateOptions{})
			if errD != nil {
				log.Printf("Failed to update deployment error: %v \n", errD)
				return errD
			}
		} else {
			// 同步inference status
			if deploy.Status.AvailableReplicas != infer.Status.AvailableReplicas {
				errI := ic.updateInferenceStatus(infer, deploy)
				if errI != nil {
					log.Printf("Failed to update inference status error: %v \n", errI)
					return errI
				}
			}
		}
	}

	service, err := ic.sLister.Services(namespace).Get(infer.Spec.Service.Name)
	if errors.IsNotFound(err) {
		newService := ic.constructService(infer)
		_, errS := ic.kubeclient.CoreV1().Services(namespace).Create(ctx, newService, metav1.CreateOptions{})
		if errS != nil {
			log.Printf("Failed to create service [%#v], error: %v\n", newService, errS)
			return errS
		}
	} else if err != nil {
		return err
	} else {
		// 判断service的containerPort和inference配置的port是否一致,默认只有一个port配置
		if len(service.Spec.Ports) == 0 || service.Spec.Ports[0].Port != infer.Spec.Deployment.ContainerPort {
			serviceCopy := service.DeepCopy()
			serviceCopy.Spec.Ports[0].Port = infer.Spec.Deployment.ContainerPort
			_, errS := ic.kubeclient.CoreV1().Services(namespace).Update(ctx, serviceCopy, metav1.UpdateOptions{})
			if errS != nil {
				log.Printf("Failed to update service [%#v], error: %v\n", serviceCopy, errS)
				return errS
			}
		}
	}

	ingress, err := ic.igLister.Ingresses(namespace).Get(infer.Spec.Ingress.Name)
	if errors.IsNotFound(err) {
		newIngress := ic.constructIngress(infer)
		_, errIg := ic.kubeclient.NetworkingV1().Ingresses(namespace).Create(ctx, newIngress, metav1.CreateOptions{})
		if errIg != nil {
			log.Printf("Failed to create ingress [%#v], error: %v\n", newIngress, errIg)
			return errIg
		}
	} else if err != nil {
		return err
	} else {
		if ic.ingressNeedUpdate(ingress, infer) {
			newIngress := ic.constructIngress(infer)
			_, errIg := ic.kubeclient.NetworkingV1().Ingresses(namespace).Update(ctx, newIngress, metav1.UpdateOptions{})
			if errIg != nil {
				log.Printf("Failed to update ingress [%#v], error: %v\n", newIngress, errIg)
				return errIg
			}
		}
	}
	return nil
}

func (ic *InferenceController) syncError(err error, key interface{}) {
	if err == nil {
		// 控制器执行逻辑没报错
		ic.queue.Forget(key)
		return
	}

	if ic.queue.NumRequeues(key) < maxRetries {
		// 没有超出重试次数，加入延迟队列
		ic.queue.AddRateLimited(key)
		return
	}

	fmt.Printf("sync inference with error, error: %v \n", err)
	ic.queue.Forget(key)
}

func (ic *InferenceController) constructDeployment(inference *crdv1.Inference) *appsv1.Deployment {
	replicas := inference.Spec.Deployment.Replicas
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(inference, inferenceKind),
			},
			Name:      inference.Spec.Deployment.Name,
			Namespace: inference.Namespace,
			Labels:    inference.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: inference.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: inference.Labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  fmt.Sprintf("%s-container", inference.Spec.Deployment.Name),
							Image: inference.Spec.Deployment.Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: inference.Spec.Deployment.ContainerPort,
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment
}

func (ic *InferenceController) constructService(inference *crdv1.Inference) *corev1.Service {
	// 默认单pod 单container
	port := inference.Spec.Deployment.ContainerPort
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(inference, inferenceKind),
			},
			Name:      inference.Spec.Service.Name,
			Namespace: inference.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: inference.Labels,
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: port,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: port,
					},
				},
			},
		},
	}
	return service
}

func (ic *InferenceController) constructIngress(inference *crdv1.Inference) *networkingv1.Ingress {
	ingressClassName := inference.Spec.Ingress.ClassName
	pathType := networkingv1.PathTypePrefix
	port := inference.Spec.Deployment.ContainerPort
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(inference, inferenceKind),
			},
			Name:      inference.Spec.Ingress.Name,
			Namespace: inference.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     inference.Spec.Ingress.UrlPath,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: inference.Spec.Service.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return ingress
}

func (ic *InferenceController) addInference(obj interface{}) {
	infer := obj.(*crdv1.Inference)
	log.Printf("Adding Inference [%s] in namespace [%s]. \n", infer.Name, infer.Namespace)
	ic.enqueue(infer)
}

func (ic *InferenceController) updateInference(oldObj interface{}, newObj interface{}) {
	oldInfer := oldObj.(*crdv1.Inference)
	newInfer := newObj.(*crdv1.Inference)
	if reflect.DeepEqual(oldInfer, newInfer) {
		log.Printf("inference [%s] in  namespace [%s] has nothing changes, no processing required.\n", oldInfer.Name, oldInfer.Namespace)
		return
	}
	ic.enqueueInference(newInfer)
}

func (ic *InferenceController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	// 无法区分是 add 还是 delete
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(*cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		} else {
			object, ok = tombstone.Obj.(metav1.Object)
			if !ok {
				runtime.HandleError(fmt.Errorf("tombstone contained object that is not a Deployment %#v", obj))
				return
			}
		}
		log.Printf("Processing object [%s] in namespace [%s] \n.", object.GetName(), object.GetNamespace())
	}

	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		infer := ic.resolveControllerRef(object.GetNamespace(), ownerRef)
		if infer == nil {
			return
		}
		log.Printf("Enqueuing object [%s] in namespace [%s]. \n", object.GetName(), object.GetNamespace())
		ic.enqueue(infer)
	}
}

func (ic *InferenceController) updateInferenceStatus(infer *crdv1.Inference, deploy *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	inferenceCopy := infer.DeepCopy()
	inferenceCopy.Status.AvailableReplicas = deploy.Status.AvailableReplicas
	_, err := ic.crdclient.CrdV1().Inferences(infer.Namespace).UpdateStatus(context.TODO(), inferenceCopy, metav1.UpdateOptions{})
	return err
}

func (ic *InferenceController) ingressNeedUpdate(ingress *networkingv1.Ingress, infer *crdv1.Inference) bool {
	if *ingress.Spec.IngressClassName != infer.Spec.Ingress.ClassName {
		return true
	}

	if len(ingress.Spec.Rules) == 0 {
		return true
	}

	if ingress.Spec.Rules[0].HTTP.Paths[0].Path != infer.Spec.Ingress.UrlPath {
		return true
	}

	return false
}
