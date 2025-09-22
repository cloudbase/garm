package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	apiClientEnterprises "github.com/cloudbase/garm/client/enterprises"
	apiClientOrgs "github.com/cloudbase/garm/client/organizations"
	apiClientRepos "github.com/cloudbase/garm/client/repositories"
	apiTemplates "github.com/cloudbase/garm/client/templates"
	"github.com/cloudbase/garm/params"
)

func resolveTemplate(nameOrID string) (float64, error) {
	if parsed, err := strconv.ParseUint(nameOrID, 10, 64); err == nil {
		return float64(parsed), nil
	}

	listTplReq := apiTemplates.NewListTemplatesParams()
	listTplReq.PartialName = &nameOrID
	response, err := apiCli.Templates.ListTemplates(listTplReq, authToken)
	if err != nil {
		return 0, fmt.Errorf("failed to list templates")
	}
	if len(response.Payload) == 0 {
		return 0, fmt.Errorf("no such template: %s", nameOrID)
	}
	exactMatches := []params.Template{}
	for _, val := range response.Payload {
		if val.Name == nameOrID {
			exactMatches = append(exactMatches, val)
		}
	}
	if len(exactMatches) > 1 {
		return 0, fmt.Errorf("multiple templates found with name: %s", nameOrID)
	}
	return float64(exactMatches[0].ID), nil
}

func resolveRepository(nameOrID, endpoint string) (string, error) {
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
	if endpoint != "" {
		listReposReq.Endpoint = &endpoint
	}
	response, err := apiCli.Repositories.ListRepos(listReposReq, authToken)
	if err != nil {
		return "", err
	}
	if len(response.Payload) == 0 {
		return "", fmt.Errorf("repository %s was not found", nameOrID)
	}

	if len(response.Payload) > 1 {
		return "", fmt.Errorf("multiple repositories with the name %s exist, please use the repository ID or specify the --endpoint parameter", nameOrID)
	}
	return response.Payload[0].ID, nil
}

func resolveOrganization(nameOrID, endpoint string) (string, error) {
	if nameOrID == "" {
		return "", fmt.Errorf("missing organization name or ID")
	}
	entityID, err := uuid.Parse(nameOrID)
	if err == nil {
		return entityID.String(), nil
	}

	listOrgsReq := apiClientOrgs.NewListOrgsParams()
	listOrgsReq.Name = &nameOrID
	if endpoint != "" {
		listOrgsReq.Endpoint = &endpoint
	}
	response, err := apiCli.Organizations.ListOrgs(listOrgsReq, authToken)
	if err != nil {
		return "", err
	}

	if len(response.Payload) == 0 {
		return "", fmt.Errorf("organization %s was not found", nameOrID)
	}

	if len(response.Payload) > 1 {
		return "", fmt.Errorf("multiple organizations with the name %s exist, please use the organization ID or specify the --endpoint parameter", nameOrID)
	}

	return response.Payload[0].ID, nil
}

func resolveEnterprise(nameOrID, endpoint string) (string, error) {
	if nameOrID == "" {
		return "", fmt.Errorf("missing enterprise name or ID")
	}
	entityID, err := uuid.Parse(nameOrID)
	if err == nil {
		return entityID.String(), nil
	}

	listEnterprisesReq := apiClientEnterprises.NewListEnterprisesParams()
	listEnterprisesReq.Name = &enterpriseName
	if endpoint != "" {
		listEnterprisesReq.Endpoint = &endpoint
	}
	response, err := apiCli.Enterprises.ListEnterprises(listEnterprisesReq, authToken)
	if err != nil {
		return "", err
	}

	if len(response.Payload) == 0 {
		return "", fmt.Errorf("enterprise %s was not found", nameOrID)
	}

	if len(response.Payload) > 1 {
		return "", fmt.Errorf("multiple enterprises with the name %s exist, please use the enterprise ID or specify the --endpoint parameter", nameOrID)
	}

	return response.Payload[0].ID, nil
}
