package main

import (
	"context"
	"log"

	"github.com/AndriyKalashnykov/authentik-k8s/gotest/internal/authentik"
	api "goauthentik.io/api/v3"
)

const AuthentikBootstrapToken = "NoMlxBQuYgfu3j19ygGqhjXenAjrJgOfN5naqmSDBUhdLjYqHKze7yyzY07H"
const GroupName = "QleetOS"
const QleetctlUser = "qleetctl"
const QleetctlUserPwd = "Qleetctl1234567890!"
const UsersPath = "users"

func main() {
	config := authentik.CreateConfiguration("https", "172.18.255.200:443", AuthentikBootstrapToken)
	apiClient := api.NewAPIClient(config)

	ctx := context.Background()

	// create a group
	// will create new QleetOS with different pk
	grp, _, err := authentik.CreateGroup(ctx, apiClient, GroupName)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	groupUID := grp.Pk
	log.Printf("groupUID``: %v\n", groupUID)

	// create a user and include it to previously created group
	usr, _, err := authentik.CreateUser(ctx, apiClient, groupUID, QleetctlUser, UsersPath)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	userUID := usr.Pk
	log.Printf("userUID``: %v\n", userUID)

	// create user's password
	resp, err := authentik.UpdateUserPassword(ctx, apiClient, userUID, QleetctlUserPwd)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	if resp != nil {

	}

	// get user's Groups
	pl, resp, err := authentik.ListUser(ctx, apiClient, QleetctlUser)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	if resp != nil {
		users := pl.GetResults()
		if pl != nil {
			log.Printf("Groups: %v", users[0].Groups)
		}
	}
}
