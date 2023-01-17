https://goauthentik.io/docs/installation/docker-compose
https://github.com/goauthentik/authentik/tags

```bash
# You can also use openssl instead: `openssl rand -base64 36`
sudo apt-get install -y pwgen
# Because of a PostgreSQL limitation, only passwords up to 99 chars are supported
# See https://www.postgresql.org/message-id/09512C4F-8CB9-4021-B455-EF4C4F0D55A0@amazon.com
echo "PG_PASS=$(pwgen -s 40 1)" >> .env
echo "AUTHENTIK_SECRET_KEY=$(pwgen -s 50 1)" >> .env
# Skip if you don't want to enable error reporting
echo "AUTHENTIK_ERROR_REPORTING__ENABLED=true" >> .env

docker-compose pull
docker-compose up

https://localhost:9443/if/flow/initial-setup/


docker-compose down --volumes
rm -rf certs/
rm -rf custom-templates/
rm -rf media/

```

# Client

https://github.com/goauthentik/terraform-provider-authentik/blob/fd45b0834275920ebccf57901d2ad4cc4bf2ef6d/internal/provider/provider.go#L230