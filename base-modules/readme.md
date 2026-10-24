### create namespace
```console
# kubectl create namespace base-modules
```

### install helm v3
``` console
# curl -fsSL -o get_helm.sh https://example.invalid/helm/helm/master/scripts/get-helm-3
# chmod 700 get_helm.sh
# ./get_helm.sh
```

### install MetalLB(Helm)
```console
# helm install metallb stable/metallb -f metallb/value.yaml --namespace base-modules
```

### install nginx-ingress and allow header underscores
```console
# helm install nginx-ingress stable/nginx-ingress -f nginx-ingress\values.yaml --namespace base-modules
# kubectl edit cm nginx-ingress-controller
```

```sh
# allow header underscores
enable-underscores-in-headers: "true"
ignore-invalid-headers: "false"
```

### install reloader
```console
# kubectl apply -f https://example.invalid/stakater/Reloader/master/deployments/kubernetes/reloader.yaml
```

### 內部域名配置 CoreDNS
> kubectl edit cm coredns -n kube-system
```sh
    # 在 health 下面插入以下文字
    # 省略了”FILE“，"ZONES"等字段
    hosts {
        # 1个hostname映射1个ip；
        # 虽然格式"0.0.0.0 0.0.0.0 0.0.0.0 kubenode1"可以正常下发configmap资源，但只有第一个ip生效
        0.0.0.0   example.invalid
        0.0.0.0   example.invalid
        0.0.0.0   example.invalid
        0.0.0.0   example.invalid
        0.0.0.0   example.invalid
        0.0.0.0   example.invalid
        0.0.0.0   example.invalid
        fallthrough
    }
```