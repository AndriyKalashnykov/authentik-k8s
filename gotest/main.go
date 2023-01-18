package main

import (
	"fmt"
	"log"
	"os"

	"github.com/AndriyKalashnykov/authentik-k8s/gotest/internal/authentik"
	api "goauthentik.io/api/v3"
)

const AuthentikBootstrapToken = "NoMlxBQuYgfu3j19ygGqhjXenAjrJgOfN5naqmSDBUhdLjYqHKze7yyzY07H"
const GroupName = "QleetOS"

func main() {
	config := authentik.CreateConfiguration("https", "172.18.255.200:443", AuthentikBootstrapToken)
	apiClient := api.NewAPIClient(config)

	// will create new QleetOS with different pk
	grp, _, err := authentik.CreateGroup(apiClient, GroupName)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	groupUID := grp.Pk
	fmt.Fprintf(os.Stdout, "groupUID``: %v\n", groupUID)

	usr, _, err := authentik.CreateUser(apiClient, groupUID)
	if err != nil {
		log.Panicf("error: %v", err)
	}
	userUID := usr.Pk
	fmt.Fprintf(os.Stdout, "userUID``: %v\n", userUID)

	//identifier := "akadmin"
	//user := 1
	//expiring := false

	//tokenRequest := api.TokenRequest{
	//	Identifier: identifier,
	//	User:       IntToPointer(user),
	//	Expiring:   BoolToPointer(expiring),
	//}
	//
	//intent := api.IntentEnum(api.INTENTENUM_API)
	//tokenRequest.Intent = &intent
	//
	//fmt.Println(tokenRequest)
}
