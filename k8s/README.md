helm repo add authentik https://charts.goauthentik.io
helm repo update

AUTHENTIK_BOOTSTRAP_PASSWORD=Authentik01234567890!
AUTHENTIK_BOOTSTRAP_TOKEN=NoMlxBQuYgfu3j19ygGqhjXenAjrJgOfN5naqmSDBUhdLjYqHKze7yyzY07H

# helm template authentik authentik/authentik -f ./values.yml > deploy.yml
# helm upgrade --install authentik authentik/authentik -f values.yml

kubectl apply -f ./deploy.yml

kubectl patch svc authentik -n default --type='json' -p "[{\"op\":\"replace\",\"path\":\"/spec/type\",\"value\":\"LoadBalancer\"}]"
echo "waiting for authentik to get External-IP"
until kubectl get service/authentik -n default --output=jsonpath='{.status.loadBalancer}' | grep "ingress"; do : ; done

LB_IP=$(kubectl get svc/authentik -o=jsonpath='{.status.loadBalancer.ingress[0].ip}')

# login: akadmin, pwd: Authentik01234567890!
xdg-open https://$LB_IP:443

#xdg-open https://$LB_IP:9443/if/flow/initial-setup/


kubectl delete -f ./deploy.yml
