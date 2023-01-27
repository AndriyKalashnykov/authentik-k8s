package main

import (
	"context"
	"log"

	"github.com/AndriyKalashnykov/authentik-k8s/gotest/internal/authentik"
	api "goauthentik.io/api/v3"
)

const AuthentikServerScheme = "https"
const AuthentikServerHost = "172.18.255.200:443"
const AuthentikBootstrapToken = "NoMlxBQuYgfu3j19ygGqhjXenAjrJgOfN5naqmSDBUhdLjYqHKze7yyzY07H" // see AUTHENTIK_BOOTSTRAP_TOKEN in K8s manifests
const QleetOSGroupName = "QleetOS"
const QleetOSGroupIsSuperUser = false // can login to Authintic admin Web UI interface
const QleetctlUser = "qleetctl"
const QleetctlUserPwd = "Qleetctl1234567890!"
const UsersPath = "users"
const QleetctlTokenIdentifier = "qleetctl-token"
const QleetctlTokenIdentifierDescription = "qleetctl-token created with authentik/go-client"

func main() {
	ctx := context.Background()

	// create authentic API client using AuthentikBootstrapToken used during Authentik deployment

	akadminConfig := authentik.CreateConfiguration(AuthentikServerScheme, AuthentikServerHost, AuthentikBootstrapToken)
	akadminApiClient := api.NewAPIClient(akadminConfig)

	// create QleetOS group
	// will create new QleetOS group with different pk
	grp, _, err := authentik.CreateGroup(ctx, akadminApiClient, QleetOSGroupName, QleetOSGroupIsSuperUser)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	groupUID := grp.Pk
	log.Printf("groupUID``: %v\n", groupUID)

	// create qleetctl user and assign it to previously created group
	usr, _, err := authentik.CreateUser(ctx, akadminApiClient, groupUID, QleetctlUser, UsersPath)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	userUID := usr.Pk
	log.Printf("userUID``: %v\n", userUID)

	// create qleetctl user's password
	resp, err := authentik.UpdateUserPassword(ctx, akadminApiClient, userUID, QleetctlUserPwd)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	if resp != nil {

	}

	// create qleetctl user's OAuth token
	token, resp, err := authentik.CreateUserToken(ctx, akadminApiClient, userUID, QleetctlTokenIdentifier, QleetctlTokenIdentifierDescription)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	if token != nil {
		log.Printf("Token: %v", token)
	}

	// retrieve qleetctl user's OAuth token
	tv, _, err := authentik.RetrieveUserToken(ctx, akadminApiClient, QleetctlTokenIdentifier)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	if tv != nil {
		log.Printf("OAuth token: %v", tv.Key)
	}

	//userToken = "ZId4CDEtmHbnuxkJH2ehUzHgYeTmOansuCO0JsTTsZnYB1z9N0WoAutpyH4i"

	// create authentic API client using qleetctl Oauth token (tv.Key) from previous step
	qleetctlConfig := authentik.CreateConfiguration(AuthentikServerScheme, AuthentikServerHost, tv.Key)
	qleetctlApiClient := api.NewAPIClient(qleetctlConfig)

	// get qleetctl own user's info
	su, _, err := authentik.MeRetrieveUser(ctx, qleetctlApiClient)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	if su != nil {
		log.Printf("User Groups: %v", su.GetUser().Groups)
	}
}
