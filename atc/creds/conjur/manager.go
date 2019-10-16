package conjur

import (
	"errors"
	"io/ioutil"
	"text/template"
	"text/template/parse"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
	"github.com/cyberark/conjur-api-go/conjurapi"
	"github.com/cyberark/conjur-api-go/conjurapi/authn"
)

const DefaultPipelineSecretTemplate = "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
const DefaultTeamSecretTemplate = "/concourse/{{.Team}}/{{.Secret}}"

type Manager struct {
	ConjurApplianceUrl     string `long:"appliance-url" description:"URL of the conjur instance"`
	ConjurAccount          string `long:"account" description:"Conjur Account"`
	ConjurCertFile         string `long:"cert-file" description:"Cert file used if conjur instance is using a self signed cert. E.g. /path/to/conjur.pem"`
	ConjurAuthnLogin       string `long:"authn-login" description:"Host username. E.g host/concourse"`
	ConjurAuthnApiKey      string `long:"authn-api-key" description:"Api key related to the host"`
	PipelineSecretTemplate string `long:"pipeline-secret-template" description:"AWS Secrets Manager secret identifier template used for pipeline specific parameter" default:"/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"`
	TeamSecretTemplate     string `long:"team-secret-template" description:"AWS Secrets Manager secret identifier  template used for team specific parameter" default:"/concourse/{{.Team}}/{{.Secret}}"`
	Conjur                 *Conjur
}

type Secret struct {
	Team     string
	Pipeline string
	Secret   string
}

func buildSecretTemplate(name, tmpl string) (*template.Template, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	if parse.IsEmptyTree(t.Root) {
		return nil, errors.New("secret template should not be empty")
	}
	return t, nil
}

func (manager *Manager) Init(log lager.Logger) error {

	config, err := conjurapi.LoadConfig()
	if err != nil {
		log.Error("load-conjur-config", err)
		return err
	}
	config.ApplianceURL = manager.ConjurApplianceUrl
	config.Account = manager.ConjurAccount

	conjur, err := conjurapi.NewClientFromKey(config,
		authn.LoginPair{
			Login:  manager.ConjurAuthnLogin,
			APIKey: manager.ConjurAuthnApiKey,
		},
	)
	if err != nil {
		log.Error("create-conjur-api-instance", err)
		return err
	}

	manager.Conjur = &Conjur{
		log:    log,
		client: conjur,
	}

	return nil
}

func (manager *Manager) Health() (*creds.HealthResponse, error) {
	health := &creds.HealthResponse{
		Method: "GetSecretValue",
	}

	health.Response = map[string]string{
		"status": "UP",
	}

	return health, nil
}

func (manager *Manager) IsConfigured() bool {
	return manager.ConjurApplianceUrl != ""
}

func (manager *Manager) Validate() error {
	// Make sure that the template is valid
	pipelineSecretTemplate, err := buildSecretTemplate("pipeline-secret-template", manager.PipelineSecretTemplate)
	if err != nil {
		return err
	}
	teamSecretTemplate, err := buildSecretTemplate("team-secret-template", manager.TeamSecretTemplate)
	if err != nil {
		return err
	}

	// Execute the templates on dummy data to verify that it does not expect additional data
	dummy := Secret{Team: "team", Pipeline: "pipeline", Secret: "secret"}
	if err = pipelineSecretTemplate.Execute(ioutil.Discard, &dummy); err != nil {
		return err
	}
	if err = teamSecretTemplate.Execute(ioutil.Discard, &dummy); err != nil {
		return err
	}

	// All of the AWS credential variables may be empty since credentials may be obtained via environemnt variables
	// or other means. However, if one of them is provided, then all of them (except session token) must be provided.
	if manager.ConjurApplianceUrl == "" && manager.ConjurAccount == "" && manager.ConjurAuthnLogin == "" && manager.ConjurAuthnApiKey == "" {
		return nil
	}

	if manager.ConjurAuthnLogin == "" {
		return errors.New("must provide conjur authn login")
	}

	if manager.ConjurAuthnApiKey == "" {
		return errors.New("must provide conjur authn key")
	}

	if manager.ConjurApplianceUrl == "" {
		return errors.New("must provide conjur appliance url")
	}

	if manager.ConjurAccount == "" {
		return errors.New("must provide conjur account")
	}

	return nil
}

func (manager *Manager) NewSecretsFactory(log lager.Logger) (creds.SecretsFactory, error) {

	config, err := conjurapi.LoadConfig()
	if err != nil {
		log.Error("load-conjur-config", err)
		return nil, err
	}
	config.ApplianceURL = manager.ConjurApplianceUrl
	config.Account = manager.ConjurAccount

	client, err := conjurapi.NewClientFromKey(config,
		authn.LoginPair{
			Login:  manager.ConjurAuthnLogin,
			APIKey: manager.ConjurAuthnApiKey,
		},
	)
	if err != nil {
		log.Error("create-conjur-api-instance", err)
		return nil, err
	}

	pipelineSecretTemplate, err := buildSecretTemplate("pipeline-secret-template", manager.PipelineSecretTemplate)
	if err != nil {
		return nil, err
	}

	teamSecretTemplate, err := buildSecretTemplate("team-secret-template", manager.TeamSecretTemplate)
	if err != nil {
		return nil, err
	}

	return NewConjurFactory(log, client, []*template.Template{pipelineSecretTemplate, teamSecretTemplate}), nil
}
