helm repo add authentik https://charts.goauthentik.io
helm repo update

AUTHENTIK_BOOTSTRAP_PASSWORD=Authentik01234567890!
AUTHENTIK_BOOTSTRAP_TOKEN=NoMlxBQuYgfu3j19ygGqhjXenAjrJgOfN5naqmSDBUhdLjYqHKze7yyzY07H

helm template authentik authentik/authentik -f ./values.yml > deploy.yml
helm upgrade --install authentik authentik/authentik -f values.yml

#xdg-open https://$LB_IP:9443/if/flow/initial-setup/

kubectl delete -f ./deploy.yml
