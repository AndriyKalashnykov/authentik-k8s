package authentik

import (
	"context"
	"fmt"
	"net/http"

	"github.com/AndriyKalashnykov/authentik-k8s/gotest/internal/util"
	"goauthentik.io/api/v3"
)

func CreateConfiguration(scheme string, host string, token string) *api.Configuration {
	config := api.NewConfiguration()
	config.Debug = true
	config.Scheme = scheme
	config.Host = host
	config.HTTPClient = &http.Client{
		Transport: util.GetTLSTransport(true),
	}

	config.AddDefaultHeader("Authorization", fmt.Sprintf("Bearer %s", token))

	return config
}

func CreateGroup(apiClient *api.APIClient, name string) (*api.Group, *http.Response, error) {

	groupRequest := api.GroupRequest{
		Name:        name,
		IsSuperuser: util.BoolToPointer(false),
	}

	return apiClient.CoreApi.CoreGroupsCreate(context.Background()).GroupRequest(groupRequest).Execute()
}

func CreateUser(apiClient *api.APIClient, groupUID string) (*api.User, *http.Response, error) {

	userRequest := api.UserRequest{
		Name:     "name",
		Username: "username",
		Groups:   []string{groupUID}, // UID
		IsActive: util.BoolToPointer(true),
		Path:     util.StringToPointer("users"),
	}

	return apiClient.CoreApi.CoreUsersCreate(context.Background()).UserRequest(userRequest).Execute()
}
