## Before Started
```
kubectl create secret docker-registry regcred --docker-server=example.invalid --docker-username=admin --docker-password=[REDACTED] -n target-dev
```

## Development
### namespace: target-dev
#### pods: 
    - gs-game
    - gs-tfserver
    - backend-portal
    - backend-api
#### services:
    - svc-websocket
    - svc-backend-portal
    - svc-backend-api
#### configmap:
    - 族繁不及備載
#### note:

# install configmap
```
kubectl create configmap game-tfserver -n target-dev --from-file=deployment\game\tfserver\configmap\
kubectl create configmap gs-api -n target-dev --from-file=configmap\gs-api\
kubectl create configmap gs-game-dfdc -n target-dev --from-file=configmap\gs-game\dfdc
kubectl create configmap gs-game-protodata -n target-dev --from-file=configmap\gs-game\protodata
kubectl create configmap gs-game-protodata-qa -n target-dev --from-file=configmap\gs-game\protodata-qa
kubectl create configmap gs-game-bjl -n target-dev --from-file=configmap\gs-game\bjl
kubectl create configmap gs-game-brnn -n target-dev --from-file=configmap\gs-game\brnn
kubectl create configmap gs-game-tgw -n target-dev --from-file=configmap\gs-game\tgw
kubectl create configmap gs-game-tbnn -n target-dev --from-file=configmap\gs-game\tbnn
kubectl create configmap gs-game-hhdz -n target-dev --from-file=configmap\gs-game\hhdz
kubectl create configmap gs-game-ald -n target-dev --from-file=configmap\gs-game\ald
kubectl create configmap gs-game-lhd -n target-dev --from-file=configmap\gs-game\lhd
kubectl create configmap gs-game-hbsl -n target-dev --from-file=configmap\gs-game\hbsl
```

# deploy gs-tfserver pod
```
kubectl apply -f deployment\game\tfserver -n target-dev
kubectl apply -f pods\gs-game\dfdc -n target-dev
kubectl apply -f pods\gs-game\bjl -n target-dev
kubectl apply -f pods\gs-game\brnn -n target-dev
kubectl apply -f pods\gs-game\tgw -n target-dev
kubectl apply -f pods\gs-game\tbnn -n target-dev
kubectl apply -f pods\gs-game\hhdz -n target-dev
kubectl apply -f pods\gs-game\ald -n target-dev
kubectl apply -f pods\gs-game\lhd -n target-dev
kubectl apply -f pods\gs-game\hbsl -n target-dev
```


```
kubectl create configmap game-apiserver -n target-dev --from-file=deployment\game\apiserver\configmap\
kubectl create configmap game-slot-protodata -n target-dev --from-file=deployment\game\slot\protodata\
kubectl create configmap game-slot-protodata-qa -n target-dev --from-file=deployment\game\slot\protodata-qa\
kubectl create configmap game-slot-ald -n target-dev --from-file=deployment\game\slot\ald\configmap\
kubectl create configmap game-slot-dfdc -n target-dev --from-file=deployment\game\slot\dfdc\configmap\
kubectl create configmap game-slot-tgw -n target-dev --from-file=deployment\game\slot\tgw\configmap\
kubectl create configmap game-poker-bjl -n target-dev --from-file=deployment\game\poker\bjl\configmap\
kubectl create configmap game-poker-brnn -n target-dev --from-file=deployment\game\poker\brnn\configmap\
kubectl create configmap game-poker-hbsl -n target-dev --from-file=deployment\game\poker\hbsl\configmap\
kubectl create configmap game-poker-hhdz -n target-dev --from-file=deployment\game\poker\hhdz\configmap\
kubectl create configmap game-poker-lhd -n target-dev --from-file=deployment\game\poker\lhd\configmap\
kubectl create configmap game-poker-tbnn -n target-dev --from-file=deployment\game\poker\tbnn\configmap\
kubectl apply -f deployment\game\slot\ald -n target-dev
kubectl apply -f deployment\game\slot\dfdc -n target-dev
kubectl apply -f deployment\game\slot\tgw -n target-dev
kubectl apply -f deployment\game\poker\bjl -n target-dev
kubectl apply -f deployment\game\poker\brnn -n target-dev
kubectl apply -f deployment\game\poker\hbsl -n target-dev
kubectl apply -f deployment\game\poker\hhdz -n target-dev
kubectl apply -f deployment\game\poker\lhd -n target-dev
kubectl apply -f deployment\game\poker\tbnn -n target-dev

```

```
kubectl delete cm game-tfserver -n target-dev
kubectl create configmap game-tfserver -n target-dev --from-file=deployment\game\tfserver\configmap\

```
kubectl create configmap backend-api -n target-dev --from-file=deployment\backend\api\configmap\
kubectl apply -f deployment\backend\api\deployment.yml
```

```
kubectl create configmap backend-portal -n target-dev --from-file=deployment\backend\portal\configmap\
kubectl apply -f deployment\backend\portal\deployment.yml
```