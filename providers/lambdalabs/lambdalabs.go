package lambdalabs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/rs/zerolog/log"
	"github.com/unweave/unweave/api"
	"github.com/unweave/unweave/providers/lambdalabs/client"
	"github.com/unweave/unweave/tools"
	"github.com/unweave/unweave/tools/random"
)

const apiURL = "https://cloud.lambdalabs.com/api/v1/"

// err400 can happen when ll doesn't have enough capacity to create the instance
func err400(msg string, err error) *api.Error {
	return &api.Error{
		Code:     400,
		Provider: api.LambdaLabsProvider,
		Message:  msg,
		Err:      err,
	}
}

func err401(msg string, err error) *api.Error {
	return &api.Error{
		Code:       401,
		Provider:   api.LambdaLabsProvider,
		Message:    msg,
		Suggestion: "Make sure your LambdaLabs credentials are up to date",
		Err:        err,
	}
}

func err403(msg string, err error) *api.Error {
	return &api.Error{
		Code:       403,
		Provider:   api.LambdaLabsProvider,
		Message:    msg,
		Suggestion: "Make sure your LambdaLabs credentials are up to date",
		Err:        err,
	}
}

func err404(msg string, err error) *api.Error {
	return &api.Error{
		Code:       404,
		Provider:   api.LambdaLabsProvider,
		Message:    msg,
		Suggestion: "",
		Err:        err,
	}
}

func err500(msg string, err error) *api.Error {
	if msg == "" {
		msg = "Unknown error"
	}
	return &api.Error{
		Code:       500,
		Message:    msg,
		Suggestion: "LambdaLabs might be experiencing issues. Check the service status page at https://status.lambdalabs.com/",
		Provider:   api.LambdaLabsProvider,
		Err:        err,
	}
}

// We return this when LambdaLabs doesn't have enough capacity to create the instance.
func err503(msg string, err error) *api.Error {
	return &api.Error{
		Code:     503,
		Provider: api.LambdaLabsProvider,
		Message:  msg,
		Err:      err,
	}
}

func errUnknown(code int, err error) *api.Error {
	return &api.Error{
		Code:       code,
		Message:    "Unknown error",
		Suggestion: "",
		Provider:   api.LambdaLabsProvider,
		Err:        err,
	}
}

type Session struct {
	client *client.ClientWithResponses
}

func (r *Session) GetProvider() api.RuntimeProvider {
	return api.LambdaLabsProvider
}

func (r *Session) AddSSHKey(ctx context.Context, sshKey api.SSHKey) (api.SSHKey, error) {
	if sshKey.Name == "" {
		return api.SSHKey{}, fmt.Errorf("SSH key name is required")
	}

	keys, err := r.ListSSHKeys(ctx)
	if err != nil {
		return api.SSHKey{}, fmt.Errorf("failed to list ssh keys, err: %w", err)
	}

	for _, k := range keys {
		if sshKey.Name == k.Name {
			// Key exists, make sure it has the same public key if provided
			if sshKey.PublicKey != k.PublicKey {
				return api.SSHKey{}, err400("SSH key with the same name already exists with a different public key", nil)
			}
			log.Ctx(ctx).Info().Msgf("SSH Key %q already exists, using existing key", sshKey.Name)
			return k, nil
		}
		if k.PublicKey == sshKey.PublicKey {
			log.Ctx(ctx).Info().Msgf("SSH Key %q already exists, using existing key", sshKey.Name)
			return k, nil
		}
	}
	// Key doesn't exist, create a new one

	log.Ctx(ctx).Info().Msgf("Generating new SSH key %q", sshKey.Name)

	req := client.AddSSHKeyJSONRequestBody{
		Name:      sshKey.Name,
		PublicKey: &sshKey.PublicKey,
	}
	res, err := r.client.AddSSHKeyWithResponse(ctx, req)
	if err != nil {
		return api.SSHKey{}, err
	}
	if res.JSON200 == nil {
		err = fmt.Errorf("failed to generate SSH key")
		if res.JSON401 != nil {
			return api.SSHKey{}, err401(res.JSON401.Error.Message, err)
		}
		if res.JSON403 != nil {
			return api.SSHKey{}, err403(res.JSON403.Error.Message, err)
		}
		if res.JSON400 != nil {
			return api.SSHKey{}, err400(res.JSON400.Error.Message, err)
		}
		return api.SSHKey{}, errUnknown(res.StatusCode(), err)
	}

	return api.SSHKey{
		Name:      res.JSON200.Data.Name,
		PublicKey: res.JSON200.Data.PublicKey,
	}, nil
}

func (r *Session) ListSSHKeys(ctx context.Context) ([]api.SSHKey, error) {
	log.Ctx(ctx).Info().Msg("Listing SSH keys")

	res, err := r.client.ListSSHKeysWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if res.JSON200 == nil {
		if res.JSON401 != nil {
			return nil, err401(res.JSON401.Error.Message, nil)
		}
		if res.JSON403 != nil {
			return nil, err403(res.JSON403.Error.Message, nil)
		}
		return nil, errUnknown(res.StatusCode(), nil)
	}

	keys := make([]api.SSHKey, len(res.JSON200.Data))
	for i, k := range res.JSON200.Data {
		k := k
		keys[i] = api.SSHKey{
			Name:      k.Name,
			PublicKey: k.PublicKey,
		}
	}
	return keys, nil
}

func (r *Session) ListNodeTypes(ctx context.Context) ([]api.NodeType, error) {
	log.Ctx(ctx).Info().Msgf("Listing instance availability")

	res, err := r.client.InstanceTypesWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if res.JSON200 == nil {
		if res.JSON401 != nil {
			return nil, err401(res.JSON401.Error.Message, nil)
		}
		if res.JSON403 != nil {
			return nil, err403(res.JSON403.Error.Message, nil)
		}
		return nil, errUnknown(res.StatusCode(), nil)
	}

	var nodeTypes []api.NodeType
	for id, data := range res.JSON200.Data {
		data := data

		it := api.NodeType{
			ID:          id,
			Name:        &data.InstanceType.Description,
			Regions:     []string{},
			Price:       &data.InstanceType.PriceCentsPerHour,
			Description: nil,
			Provider:    api.LambdaLabsProvider,
			Specs: api.NodeSpecs{
				VCPUs:     data.InstanceType.Specs.Vcpus,
				Memory:    data.InstanceType.Specs.MemoryGib,
				GPUMemory: nil,
			},
		}

		for _, region := range data.RegionsWithCapacityAvailable {
			region := region
			it.Regions = append(it.Regions, region.Name)
		}
		nodeTypes = append(nodeTypes, it)
	}

	return nodeTypes, nil
}

func (r *Session) finRegionForNode(ctx context.Context, nodeTypeID string) (string, error) {
	nodeTypes, err := r.ListNodeTypes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list instance availability, err: %w", err)
	}

	for _, nt := range nodeTypes {
		if nt.ID == nodeTypeID {
			if len(nt.Regions) == 0 {
				continue
			}
			return nt.Regions[0], nil
		}
	}
	suggestion := ""
	b, err := json.Marshal(nodeTypes)
	if err != nil {
		log.Ctx(ctx).Warn().Msgf("Failed to marshal available instances to JSON: %v", err)
	} else {
		suggestion += string(b)
	}

	e := err503(fmt.Sprintf("No region with available capacity for node type %q", nodeTypeID), nil)
	e.Suggestion = suggestion
	return "", e
}

func (r *Session) InitNode(ctx context.Context, sshKey api.SSHKey, nodeTypeID string, region *string) (api.Node, error) {
	log.Ctx(ctx).Info().Msgf("Launching instance with SSH key %q", sshKey.Name)

	if region == nil {
		var err error
		var nr string
		nr, err = r.finRegionForNode(ctx, nodeTypeID)
		if err != nil {
			return api.Node{}, err
		}
		region = &nr
	}

	req := client.LaunchInstanceJSONRequestBody{
		FileSystemNames:  nil,
		InstanceTypeName: nodeTypeID,
		Name:             tools.Stringy("uw-" + random.GenerateRandomPhrase(3, "-")),
		Quantity:         tools.Inty(1),
		RegionName:       *region,
		SshKeyNames:      []string{sshKey.Name},
	}

	res, err := r.client.LaunchInstanceWithResponse(ctx, req)
	if err != nil {
		return api.Node{}, err
	}
	if res.JSON200 == nil {
		if res.JSON401 != nil {
			return api.Node{}, err401(res.JSON401.Error.Message, nil)
		}
		if res.JSON403 != nil {
			return api.Node{}, err403(res.JSON403.Error.Message, nil)
		}
		if res.JSON500 != nil {
			return api.Node{}, err500(res.JSON500.Error.Message, nil)
		}
		if res.JSON404 != nil {
			return api.Node{}, err404(res.JSON404.Error.Message, nil)
		}

		// We get a 400 if the instance type is not available. We check for the available
		// instances and return them in the error message. Since this is not critical, we
		// can ignore if there's any errors in the process.
		if res.JSON400 != nil {
			suggestion := ""
			msg := strings.ToLower(res.JSON400.Error.Message)
			if strings.Contains(msg, "available capacity") {
				// Get a list of available instances
				instances, e := r.ListNodeTypes(ctx)
				if e != nil {
					// Log and continue
					log.Ctx(ctx).Warn().
						Msgf("Failed to get a list of available instances: %v", e)
				}

				b, e := json.MarshalIndent(instances, "", "  ")
				if e != nil {
					log.Ctx(ctx).Warn().
						Msgf("Failed to marshal available instances to JSON: %v", e)
				} else {
					suggestion += string(b)
				}
				err := err503(res.JSON400.Error.Message, nil)
				err.Suggestion = suggestion
				return api.Node{}, err
			}
			return api.Node{}, err400(res.JSON400.Error.Message, nil)
		}

		return api.Node{}, errUnknown(res.StatusCode(), err)
	}

	if len(res.JSON200.Data.InstanceIds) == 0 {
		return api.Node{}, fmt.Errorf("failed to launch instance")
	}

	return api.Node{
		ID:       res.JSON200.Data.InstanceIds[0],
		TypeID:   nodeTypeID,
		Region:   *region,
		KeyPair:  sshKey,
		Status:   api.StatusInitializing,
		Provider: api.LambdaLabsProvider,
	}, nil
}

func (r *Session) TerminateNode(ctx context.Context, nodeID string) error {
	log.Ctx(ctx).Info().Msgf("Terminating instance %q", nodeID)

	req := client.TerminateInstanceJSONRequestBody{
		InstanceIds: []string{nodeID},
	}
	res, err := r.client.TerminateInstanceWithResponse(ctx, req)
	if err != nil {
		return err
	}
	if res.JSON200 == nil {
		if res.JSON401 != nil {
			return err401(res.JSON401.Error.Message, nil)
		}
		if res.JSON403 != nil {
			return err403(res.JSON403.Error.Message, nil)
		}
		if res.JSON400 != nil {
			return err400(res.JSON400.Error.Message, nil)
		}
		if res.JSON404 != nil {
			return err404(res.JSON404.Error.Message, nil)
		}
		if res.JSON500 != nil {
			return err500(res.JSON500.Error.Message, nil)
		}
		return errUnknown(res.StatusCode(), err)
	}

	return nil
}

func NewSessionProvider(apiKey string) (*Session, error) {
	bearerTokenProvider, err := securityprovider.NewSecurityProviderBearerToken(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create bearer token provider, err: %v", err)
	}

	llClient, err := client.NewClientWithResponses(apiURL, client.WithRequestEditorFn(bearerTokenProvider.Intercept))
	if err != nil {
		return nil, fmt.Errorf("failed to create client, err: %v", err)
	}

	return &Session{client: llClient}, nil
}
