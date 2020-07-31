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
	Name   string `yaml:"name"`
	Labels Labels `yaml:"labels"`

	NseName string `yaml:"nseName"` //TODO temporary in order to be able to run examples

	WCM *WCM `yaml:"wcm"`

	VL3 VL3 `yaml:"vl3"`
}

type WCM struct {
	Name               string `yaml:"name"`
	Address            string `yaml:"address"`
	AccessToken        string `yaml:"accessToken"`
	ConnectivityDomain string `yaml:"connectivityDomain"`
}

type VL3 struct {
	IPAM        IPAM     `yaml:"ipam"`
	Ifname      string   `yaml:"ifName"`
	NameServers []string `yaml:"nameServers"`
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
