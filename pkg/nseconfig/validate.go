package nseconfig

import (
	"bytes"
	"fmt"
	"net"
	"os"
)

type InvalidConfigErrors []error

func (v InvalidConfigErrors) Error() string {
	b := bytes.NewBufferString("validation failed with errors: \n")
	for _, err := range v {
		fmt.Fprintf(b, "\t%s\n", err)
	}
	return b.String()
}

func (c *NseControl) validate() error {
	if c == nil {
		return nil
	}
	var errs InvalidConfigErrors
	if empty(c.Address) {
		errs = append(errs, fmt.Errorf("nseControl address is not set"))
	}
	if empty(c.Name) {
		errs = append(errs, fmt.Errorf("nseControl name is not set"))
	}
	if empty(c.ConnectivityDomain) {
		errs = append(errs, fmt.Errorf("connectivity domain is not set"))
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (v VL3) validate() error {
	var errs InvalidConfigErrors

	if _, _, err := net.ParseCIDR(v.IPAM.DefaultPrefixPool); err != nil {
		errs = append(errs, fmt.Errorf("prefix pool is not a valid subnet: %s", err))
	}
	for i, r := range v.IPAM.Routes {
		if _, _, err := net.ParseCIDR(r); err != nil {
			errs = append(errs, fmt.Errorf("route nr %d with value %s is not a valid subnet: %s", i, r, err))
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (p PassThrough) validate() error {
	var errs InvalidConfigErrors

	if _, _, err := net.ParseCIDR(p.IPAM.DefaultPrefixPool); err != nil {
		errs = append(errs, fmt.Errorf("prefix pool is not a valid subnet: %s", err))
	}
	for i, r := range p.IPAM.Routes {
		if _, _, err := net.ParseCIDR(r); err != nil {
			errs = append(errs, fmt.Errorf("route nr %d with value %s is not a valid subnet: %s", i, r, err))
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (e Endpoint) validate() error {
	var errs InvalidConfigErrors
	if len(errs) > 0 {
		return errs
	}

	var validateErrors []error

	passThrough, ok := os.LookupEnv("PASS_THROUGH")
	if ok && passThrough == "true" {
		validateErrors = append(validateErrors, e.PassThrough.validate())
	} else {
		validateErrors = append(validateErrors, e.NseControl.validate(), e.VL3.validate())
	}

	for _, err := range validateErrors{
		if err != nil {
			if verr, ok := err.(InvalidConfigErrors); ok {
				errs = append(errs, verr...)
			} else {
				errs = append(errs, err)
			}
		}
	}
	if e.NseControl == nil {
		e.NseControl = &NseControl{}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (c Config) validate() error {
	if len(c.Endpoints) == 0 {
		return fmt.Errorf("no endpoints provided")
	}

	var errs InvalidConfigErrors

	for _, endp := range c.Endpoints {
		if err := endp.validate(); err != nil {
			if verr, ok := err.(InvalidConfigErrors); ok {
				errs = append(errs, verr...)
			} else {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}
