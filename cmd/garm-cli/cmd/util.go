package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	apiClientEnterprises "github.com/cloudbase/garm/client/enterprises"
	apiClientOrgs "github.com/cloudbase/garm/client/organizations"
	apiClientRepos "github.com/cloudbase/garm/client/repositories"
)

func resolveRepository(nameOrID string) (string, error) {
	if nameOrID == "" {
		return "", fmt.Errorf("missing repository name or ID")
	}
	entityID, err := uuid.Parse(nameOrID)
	if err == nil {
		return entityID.String(), nil
	}

	parts := strings.SplitN(nameOrID, "/", 2)
	if len(parts) < 2 {
		// format of friendly name is invalid for a repository.
		// Return the string as is.
		return nameOrID, nil
	}

	listReposReq := apiClientRepos.NewListReposParams()
	listReposReq.Owner = &parts[0]
	listReposReq.Name = &parts[1]
	response, err := apiCli.Repositories.ListRepos(listReposReq, authToken)
	if err != nil {
		return "", err
	}
	if len(response.Payload) == 0 {
		return "", fmt.Errorf("repository %s was not found", nameOrID)
	}

	if len(response.Payload) > 1 {
		return "", fmt.Errorf("multiple repositories with the name %s exist, please use the repository ID", nameOrID)
	}
	return response.Payload[0].ID, nil
}

func resolveOrganization(nameOrID string) (string, error) {
	if nameOrID == "" {
		return "", fmt.Errorf("missing organization name or ID")
	}
	entityID, err := uuid.Parse(nameOrID)
	if err == nil {
		return entityID.String(), nil
	}

	listOrgsReq := apiClientOrgs.NewListOrgsParams()
	listOrgsReq.Name = &nameOrID
	response, err := apiCli.Organizations.ListOrgs(listOrgsReq, authToken)
	if err != nil {
		return "", err
	}

	if len(response.Payload) == 0 {
		return "", fmt.Errorf("organization %s was not found", nameOrID)
	}

	if len(response.Payload) > 1 {
		return "", fmt.Errorf("multiple organizations with the name %s exist, please use the organization ID", nameOrID)
	}

	return response.Payload[0].ID, nil
}

func resolveEnterprise(nameOrID string) (string, error) {
	if nameOrID == "" {
		return "", fmt.Errorf("missing enterprise name or ID")
	}
	entityID, err := uuid.Parse(nameOrID)
	if err == nil {
		return entityID.String(), nil
	}

	listEnterprisesReq := apiClientEnterprises.NewListEnterprisesParams()
	listEnterprisesReq.Name = &enterpriseName
	listEnterprisesReq.Endpoint = &enterpriseEndpoint
	response, err := apiCli.Enterprises.ListEnterprises(listEnterprisesReq, authToken)
	if err != nil {
		return "", err
	}

	if len(response.Payload) == 0 {
		return "", fmt.Errorf("enterprise %s was not found", nameOrID)
	}

	if len(response.Payload) > 1 {
		return "", fmt.Errorf("multiple enterprises with the name %s exist, please use the enterprise ID", nameOrID)
	}

	return response.Payload[0].ID, nil
}
