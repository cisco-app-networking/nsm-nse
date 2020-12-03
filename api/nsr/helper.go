package nsr

import (
	"bytes"
	"fmt"
)

type validationErrors []error

func (e validationErrors) Error() string {
	b := bytes.NewBufferString("")
	for _, err := range e {
		_, _ = fmt.Fprintf(b, "\t %s", err)
	}
	return b.String()
}

func (r *NseRequest) Validate() error {
	var errs validationErrors
	if r.Address == "" {
		errs = append(errs, fmt.Errorf("NSR address is not valid"))
	}
	if r.NetworkService == "" {
		errs = append(errs, fmt.Errorf("network service name is not valid"))
	}
	if len(errs) != 0 {
		return errs
	}
	return nil
}