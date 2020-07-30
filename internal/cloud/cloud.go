package cloud

import (
	"context"
)

type Cloud interface {
	CreateContainer(ctx context.Context, image, name string, volumes []string) (Container, error)
	DestroyContainer(ctx context.Context, container Container) error
	PullImage(
		ctx context.Context,
		reference string,
		callback OutputConsumer,
		pullConfig []PullConfig,
	) error
	Exec(ctx context.Context, container Container, config ExecConfig, callback OutputConsumer) error
	Cleanup(ctx context.Context) error
	GetImageWithTag(ctx context.Context, tag string) (*Image, error)
}

type Container interface {
	String() string
	ID() string
}

type (
	OutputConsumer func(string)
	PromptConsumer func([]string)
)

type ExecConfig struct {
	Cmd          []string
	Env          []string
	WorkingDir   string
	AttachStdout bool
	AttachStderr bool
}

type Image struct {
	ID   string
	Tags []string
}

type PullConfig struct {
	Auths map[string]AuthConfig `json:"auths"`
}

type AuthConfig struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`

	ServerAddress string `json:"serveraddress,omitempty"`

	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`

	// RegistryToken is a bearer token to be sent to a registry
	RegistryToken string `json:"registrytoken,omitempty"`
}
