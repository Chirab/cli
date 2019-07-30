package scm_integrations

import (
	"gopkg.in/errgo.v1"

	"github.com/Scalingo/cli/config"
	"github.com/Scalingo/cli/io"
)

func Destroy(id string) error {
	c, err := config.ScalingoClient()
	if err != nil {
		return errgo.Notef(err, "fail to get Scalingo client")
	}

	integration, err := c.IntegrationsShow(id)
	if err != nil {
		return errgo.Notef(err, "not linked SCM integration or unknown SCM integration")
	}

	err = c.IntegrationsDestroy(id)
	if err != nil {
		return errgo.Notef(err, "fail to destroy SCM integration")
	}

	io.Statusf("Your Scalingo account and your '%s' account are unlinked.\n", integration.ScmType)
	return nil
}
