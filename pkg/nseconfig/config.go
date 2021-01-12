package nseconfig

import (
	"strings"
)

type config interface {
	validate() error
}

type Config struct {
	Endpoints []*Endpoint
}

type Endpoint struct {
	Name   		string 		`yaml:"name"`
	Labels 		Labels 		`yaml:"labels"`
	NseName 	string 		`yaml:"nseName"` //TODO temporary in order to be able to run examples
	NseControl 	*NseControl 	`yaml:"nseControl"`
	VL3 		VL3 		`yaml:"vl3"`
	AwsTgwEndpoint *AwsTgwEndpoint 	`yaml:"awsTgwEndpoint"`
}

// fields required for nse to attach to an AWS TGW
type AwsTgwEndpoint struct {
	// TODO: this is not an exhaustive list, revise this list of fields as
	// the AWS TGW connector feature gets fleshed out
	Region                  string  `yaml:"region"`
	SubnetID                string  `yaml:"subnetID"`
	TransitGatewayID 	string	`yaml:"transitGatewayID"`
	TransitGatewayName 	string	`yaml:"transitGatewayName"`
	VpcID                   string  `yaml:"vpcID"`
}

type NseControl struct {
	Name               string `yaml:"name"`
	Address            string `yaml:"address"`
	AccessToken        string `yaml:"accessToken"`
	ConnectivityDomain string `yaml:"connectivityDomain"`
}

type VL3 struct {
	IPAM        IPAM     `yaml:"ipam"`
	Ifname      string   `yaml:"ifName"`
	NameServers []string `yaml:"nameServers"`
	DNSZones    []string `yaml:"dnsZones"`
}

type IPAM struct {
	DefaultPrefixPool string   `yaml:"defaultPrefixPool"`
	PrefixLength      int      `yaml:"prefixLength"`
	Routes            []string `yaml:"routes"`
	ServerAddress     string   `yaml:"serverAddress"`
}

type decoder interface {
	Decode(v interface{}) error
}

type DecoderFn func(v interface{}) error

func (d DecoderFn) Decode(v interface{}) error { return d(v) }

func NewConfig(decoder decoder, cfg config) error {
	if err := decoder.Decode(cfg); err != nil {
		return err
	}

	return cfg.validate()
}

func empty(s string) bool {
	return len(strings.Trim(s, " ")) == 0
}
