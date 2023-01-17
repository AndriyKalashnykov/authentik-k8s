helm repo add authentik https://charts.goauthentik.io
helm repo update
helm upgrade --install authentik authentik/authentik -f values.yml

kubectl create ingress demo-localhost --class=nginx --rule="authentik.domain.tld/*=authentik:80"

kubectl patch svc ak-outpost-authentik-embedded-outpost -n default --type='json' -p "[{\"op\":\"replace\",\"path\":\"/spec/type\",\"value\":\"LoadBalancer\"}]"
echo "waiting for ak-outpost-authentik-embedded-outpost to get External-IP"
until kubectl get service/ak-outpost-authentik-embedded-outpost -n default --output=jsonpath='{.status.loadBalancer}' | grep "ingress"; do : ; done

LB_IP=$(kubectl get svc/ak-outpost-authentik-embedded-outpost -o=jsonpath='{.status.loadBalancer.ingress[0].ip}')
xdg-open https://$LB_IP:9443/if/flow/initial-setup/