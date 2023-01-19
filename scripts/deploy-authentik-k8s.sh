#!/bin/bash

LAUNCH_DIR=$(pwd); SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; cd $SCRIPT_DIR; cd ..; SCRIPT_PARENT_DIR=$(pwd);

cd $SCRIPT_PARENT_DIR

kubectl apply -f ./k8s/deploy.yml

echo "getting an External IP for Authentic svc"
kubectl patch svc authentik -n default --type='json' -p "[{\"op\":\"replace\",\"path\":\"/spec/type\",\"value\":\"LoadBalancer\"}]"
echo "waiting for authentik to get External-IP"
until kubectl get service/authentik -n default --output=jsonpath='{.status.loadBalancer}' | grep "ingress"; do : ; done

echo "waiting for authentik-worker(s) to get ready"
kubectl wait deployment -n default authentik-worker --for condition=Available=True --timeout=180s
echo "waiting for authentik-server to get ready"
kubectl wait deployment -n default authentik-server --for condition=Available=True --timeout=180s

LB_IP=$(kubectl get svc/authentik -o=jsonpath='{.status.loadBalancer.ingress[0].ip}')

echo "login: akadmin, pwd: Authentik01234567890!"

cd $LAUNCH_DIR

xdg-open https://$LB_IP:443