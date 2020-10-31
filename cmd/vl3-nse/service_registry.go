package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"cisco-app-networking.github.io/nsm-nse/api/serviceregistry"
	"cisco-app-networking.github.io/nsm-nse/pkg/metrics"
	"github.com/networkservicemesh/networkservicemesh/pkg/tools"
)

const (
	POD_NAME     = "podName"
	SERVICE_NAME = "service"
	PORT         = "port"
	CLUSTER_NAME = "clusterName"
)

type validationErrors []error

func NewServiceRegistry(addr string, ctx context.Context) (ServiceRegistry, ServiceRegistryClient, error) {
	opts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
	}

	insecure, err := strconv.ParseBool(os.Getenv(tools.InsecureEnv))
	if err != nil {
		logrus.Info("Missing INSECURE env variable. Continuing with insecure mode.")
		insecure = true
	}

	if !insecure && tools.GetConfig().SecurityProvider != nil {
		if tlsConfig, err := tools.GetConfig().SecurityProvider.GetTLSConfig(ctx); err != nil {
			logrus.Errorf(
				"Failed getting tls config with error: %v. GRPC connection will be insecure.",
				err,
			)
			opts = append(opts, grpc.WithInsecure())
		} else {
			logrus.Info("GRPC connection will be secured.")
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		}
	} else {
		logrus.Info("GRPC connection will be insecure.")
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect to ServiceRegistry: %w", err)
	}

	registryClient := serviceregistry.NewRegistryClient(conn)
	serviceRegistry := serviceRegistry{registryClient: registryClient, connection: conn}

	return &serviceRegistry, &serviceRegistry, nil
}

type ServiceRegistry interface {
	RegisterWorkload(ctx context.Context, workloadLabels map[string]string, connDom string, ipAddr []string) error
	RemoveWorkload(ctx context.Context, workloadLabels map[string]string, connDom string, ipAddr []string) error
}

type ServiceRegistryClient interface {
	Stop()
}

type serviceRegistry struct {
	registryClient serviceregistry.RegistryClient
	connection     *grpc.ClientConn
}

func (s *serviceRegistry) RegisterWorkload(ctx context.Context, workloadLabels map[string]string, connDom string, ipAddr []string) error {
	ports, err := processPortsFromLabel(workloadLabels[PORT], ";")
	if err != nil {
		logrus.Error(err)
		return err
	}

	workloadIdentifier := &serviceregistry.WorkloadIdentifier{
		Cluster: workloadLabels[CLUSTER_NAME],
		PodName: workloadLabels[POD_NAME],
		Name:    workloadLabels[SERVICE_NAME],
	}

	workload := &serviceregistry.Workload{
		Identifier: workloadIdentifier,
		IPAddress:  ipAddr,
	}

	workloads := []*serviceregistry.Workload{workload}
	serviceWorkload := &serviceregistry.ServiceWorkload{
		ServiceName:        workloadLabels[SERVICE_NAME],
		ConnectivityDomain: connDom,
		Workloads:          workloads,
		Ports:              ports,
	}

	logrus.Infof("Sending workload register request: %v", serviceWorkload)
	_, err = s.registryClient.RegisterWorkload(ctx, serviceWorkload)
	if err != nil {
		logrus.Errorf("service registration not successful: %w", err)
		return err
	}

	go func() {
		metrics.ActiveWorkloadCount.Inc()
	}()

	return nil
}

func (s *serviceRegistry) RemoveWorkload(ctx context.Context, workloadLabels map[string]string, connDom string, ipAddr []string) error {
	ports, err := processPortsFromLabel(workloadLabels[PORT], ";")
	if err != nil {
		logrus.Error(err)
		return err
	}

	workloadIdentifier := &serviceregistry.WorkloadIdentifier{
		Cluster: workloadLabels[CLUSTER_NAME],
		PodName: workloadLabels[POD_NAME],
		Name:    workloadLabels[SERVICE_NAME],
	}

	workload := &serviceregistry.Workload{
		Identifier: workloadIdentifier,
		IPAddress:  ipAddr,
	}

	workloads := []*serviceregistry.Workload{workload}
	serviceWorkload := &serviceregistry.ServiceWorkload{
		ServiceName:        workloadLabels[SERVICE_NAME],
		ConnectivityDomain: connDom,
		Workloads:          workloads,
		Ports:              ports,
	}

	logrus.Infof("Sending workload remove request: %v", serviceWorkload)
	_, err = s.registryClient.RemoveWorkload(ctx, serviceWorkload)
	if err != nil {
		logrus.Errorf("service removal not successful: %w", err)
		return err
	}

	go func() {
		metrics.ActiveWorkloadCount.Dec()
	}()

	return nil
}

func (s *serviceRegistry) Stop() {
	s.connection.Close()
}

func processPortsFromLabel(portLabel, separator string) ([]int32, error) {
	ports := strings.Split(portLabel, separator)
	servicePorts := []int32{}
	for _, port := range ports {
		portToInt, err := strconv.ParseInt(port, 10, 32)
		if err != nil {
			return nil, err
		}
		servicePorts = append(servicePorts, int32(portToInt))
	}

	return servicePorts, nil
}

func processWorkloadIps(workloadIps, separator string) []string {
	ips := strings.Split(workloadIps, separator)
	serviceIps := []string{}
	serviceIps = append(serviceIps, ips...)

	return serviceIps
}

func ValidateInLabels(labels map[string]string) validationErrors {
	var errs validationErrors
	if labels[CLUSTER_NAME] == "" {
		errs = append(errs, fmt.Errorf("cluster name not found on labels"))
	}
	if labels[SERVICE_NAME] == "" {
		errs = append(errs, fmt.Errorf("serviceName not found on labels"))
	}
	if labels[PORT] == "" {
		errs = append(errs, fmt.Errorf("ports not found on labels"))
	}
	if labels[POD_NAME] == "" {
		errs = append(errs, fmt.Errorf("pod name not found on labels"))
	}
	if len(errs) != 0 {
		return errs
	}
	return nil
}
