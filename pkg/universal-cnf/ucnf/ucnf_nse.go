package ucnf

import (
	"context"
	"os"

	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/cisco-app-networking/nsm-nse/pkg/universal-cnf/config"
	"github.com/davecgh/go-spew/spew"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

//
type UcnfNse struct {
	processEndpoints *config.ProcessEndpoints
}

func (ucnf *UcnfNse) Cleanup() {
	ucnf.processEndpoints.Cleanup()
}

func NewUcnfNse(configPath string, verify bool, backend config.UniversalCNFBackend, ceAddons config.CompositeEndpointAddons, ctx context.Context) *UcnfNse {

	cnfConfig := &nseconfig.Config{}
	f, err := os.Open(configPath)
	if err != nil {
		logrus.Fatal(err)
	}

	defer func() {
		err = f.Close()
		if err != nil {
			logrus.Errorf("closing file failed %v", err)
		}
	}()

	decodedYAML := yaml.NewDecoder(f)
	decodedYAML.SetStrict(true) // enable strict decoding to help debug potential issues for decoding YAMLs
	err = nseconfig.NewConfig(decodedYAML, cnfConfig)
	if err != nil {
		logrus.Fatal("NSE config errors: %v", err)
	}

	if err := backend.NewUniversalCNFBackend(); err != nil {
		logrus.Fatal(err)
	}

	if verify {
		spew.Dump(cnfConfig)
		return nil
	}

	configuration := common.FromEnv()

	pe := config.NewProcessEndpoints(backend, cnfConfig.Endpoints, configuration, ceAddons, ctx)

	ucnfnse := &UcnfNse{
		processEndpoints: pe,
	}

	logrus.Infof("Starting endpoints")

	if err := pe.Process(); err != nil {
		logrus.Fatalf("Error processing the new endpoints: %v", err)
	}
	return ucnfnse
}
