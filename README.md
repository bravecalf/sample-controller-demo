# sample-controller-demo
🚀一个用来深入学习并实战k8s controller的项目。

## 一、测试crd client 调用
1. 环境部署 minikube + nginx

     [minikube部署](https://minikube.sigs.k8s.io/docs/start/)

     [nginx安装部署](https://kubernetes.github.io/ingress-nginx/deploy/)
2. 安装code-generator,生成对应的clientset、lister、informer等
```shell
cd $GOPATH/src
git clone https://github.com/kubernetes/sample-controller.git
../github.com/code-generator/generate-groups.sh all sample-controller-demo/pkg/generated sample-controller-demo/pkg/apis "crd.example.com:v1"  --output-base=$GOPATH/src --go-header-file=hack/boilerplate.go.txt
```
3. 注册crd并创建对应的my-inference
```shell
kubectl apply -f examples/crd.yaml
kubectl apply -f examples/my-inference.yaml
```
4. 测试获取对应的inference
```go
运行client-demo/crd_demo.go中的CrdDemoTest方法
```
