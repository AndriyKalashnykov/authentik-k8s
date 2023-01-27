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

func CreateGroup(ctx context.Context, apiClient *api.APIClient, name string, isSuperuser bool) (*api.Group, *http.Response, error) {

	return apiClient.CoreApi.CoreGroupsCreate(ctx).GroupRequest(api.GroupRequest{
		Name:        name,
		IsSuperuser: util.BoolToPointer(isSuperuser),
	}).Execute()
}

func CreateUser(ctx context.Context, apiClient *api.APIClient, groupUID string, userName string, path string) (*api.User, *http.Response, error) {

	return apiClient.CoreApi.CoreUsersCreate(ctx).UserRequest(api.UserRequest{
		Name:     userName,
		Username: userName,
		Groups:   []string{groupUID}, // UID
		IsActive: util.BoolToPointer(true),
		Path:     util.StringToPointer(path),
	}).Execute()
}

func UpdateUserPassword(ctx context.Context, apiClient *api.APIClient, userID int32, pwd string) (*http.Response, error) {

	passwordRequest := apiClient.CoreApi.CoreUsersSetPasswordCreate(ctx, userID).UserPasswordSetRequest(api.UserPasswordSetRequest{
		Password: pwd,
	})

	return apiClient.CoreApi.CoreUsersSetPasswordCreateExecute(passwordRequest)
}

func CreateUserToken(ctx context.Context, apiClient *api.APIClient, userID int32, tokenIdentifier string, tokenDescription string) (*api.Token, *http.Response, error) {
	intent := api.IntentEnum(api.INTENTENUM_API)

	tr := api.TokenRequest{
		Identifier:  tokenIdentifier,
		Intent:      &intent,
		User:        util.Int32ToPointer(userID),
		Description: util.StringToPointer(tokenDescription),
		Expiring:    util.BoolToPointer(false),
	}

	return apiClient.CoreApi.CoreTokensCreate(ctx).TokenRequest(tr).Execute()
}

func UpdateUserToken(ctx context.Context, apiClient *api.APIClient, tokenIdentifier string, key string) (*http.Response, error) {
	return apiClient.CoreApi.CoreTokensSetKeyCreate(ctx, tokenIdentifier).TokenSetKeyRequest(api.TokenSetKeyRequest{
		Key: key,
	}).Execute()
}

func RetrieveUserToken(ctx context.Context, apiClient *api.APIClient, tokenIdentifier string) (*api.TokenView, *http.Response, error) {
	return apiClient.CoreApi.CoreTokensViewKeyRetrieveExecute(apiClient.CoreApi.CoreTokensViewKeyRetrieve(ctx, tokenIdentifier))
}

func ListUser(ctx context.Context, apiClient *api.APIClient, userName string) (*api.PaginatedUserList, *http.Response, error) {
	return apiClient.CoreApi.CoreUsersList(ctx).Username(userName).Execute()
}

func MeRetrieveUser(ctx context.Context, apiClient *api.APIClient) (*api.SessionUser, *http.Response, error) {
	return apiClient.CoreApi.CoreUsersMeRetrieveExecute(apiClient.CoreApi.CoreUsersMeRetrieve(ctx))
}
