package app

type Config struct {
	RootDir string `envconfig:"SECRETS_ROOT_DIR" required:"true" default:"/Users/duffleman/Source/infra-secrets/certificates"`
}
