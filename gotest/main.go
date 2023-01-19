package main

import (
	"context"
	"fmt"
	"log"
	"os"

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

	// will create new QleetOS with different pk
	grp, _, err := authentik.CreateGroup(ctx, apiClient, GroupName)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	groupUID := grp.Pk
	fmt.Fprintf(os.Stdout, "groupUID``: %v\n", groupUID)

	usr, _, err := authentik.CreateUser(ctx, apiClient, groupUID, QleetctlUser, UsersPath)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	userUID := usr.Pk
	fmt.Fprintf(os.Stdout, "userUID``: %v\n", userUID)

	resp, err := authentik.UpdateUserPassword(ctx, apiClient, userUID, QleetctlUserPwd)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	if resp != nil {

	}

}
